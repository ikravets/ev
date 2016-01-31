// Copyright (c) Ilia Kravets, 2014-2016. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"github.com/ikravets/errs"

	"my/ev/packet"
)

const (
	OA_UNKNOWN = iota
	OA_OPTIONS
	OA_ORDERS
	OA_BOOKS
)

type SimOperation interface {
	GetMessage() *SimMessage
	GetOptionId() packet.OptionId
	GetOrigOrderId() packet.OrderId
	GetSide() packet.MarketSide
	GetDefaultSizeDelta() int
	GetNewSize(SizeKind) int
	GetPrice() int
	CanAffect(what int) bool
	getOperation() *Operation
}

type Operation struct {
	m           *SimMessage
	sim         Sim
	origOrderId packet.OrderId
	origOrder   *order
	sibling     SimOperation
}

func (op *Operation) GetMessage() *SimMessage {
	return op.m
}
func (op *Operation) GetOrigOrderId() packet.OrderId {
	return op.origOrderId
}
func (op *Operation) populate() {
	if op.origOrder != nil {
		return
	}
	if op.sibling != nil {
		op.sibling.getOperation().populate()
		op.origOrder = op.sibling.getOperation().origOrder
	} else if op.origOrderId != packet.OrderIdUnknown {
		if ord, err := op.sim.OrderDb().findOrder(op.m.Session, op.origOrderId); err == nil {
			op.origOrder = &ord
		}
	}
}
func (op *Operation) origOrderIndex() orderIndex {
	return newOrderIndex(op.sim, op.m.Session, op.origOrderId)
}
func (o *Operation) getOptionId() (oid packet.OptionId) {
	o.populate()
	if o.origOrder != nil {
		return o.origOrder.OptionId
	} else {
		return packet.OptionIdUnknown
	}
}
func (o *Operation) getSide() (side packet.MarketSide) {
	o.populate()
	if o.origOrder != nil {
		side = o.origOrder.Side
	}
	return
}

type OperationAdd struct {
	Operation
	order
}

func (op *OperationAdd) CanAffect(what int) bool {
	return (what == OA_BOOKS || what == OA_ORDERS) && op.GetOptionId().Valid()
}
func (o *OperationAdd) getOperation() *Operation {
	return &o.Operation
}
func (o *OperationAdd) Independent() bool {
	return o.OptionId.Valid()
}
func (o *OperationAdd) GetOptionId() packet.OptionId {
	if o.OptionId.Valid() {
		return o.OptionId
	} else {
		return o.Operation.getOptionId()
	}
}
func (o *OperationAdd) GetSide() (side packet.MarketSide) {
	if o.Side != packet.MarketSideUnknown {
		return o.Side
	} else {
		return o.Operation.getSide()
	}
}
func (o *OperationAdd) GetPrice() int {
	return packet.PriceTo4Dec(o.Price)
}
func (o *OperationAdd) GetDefaultSizeDelta() int {
	return o.Size
}
func (o *OperationAdd) GetNewSize(sk SizeKind) int {
	errs.Check(sk == SizeKindDefault)
	return o.Size
}
func (op *OperationAdd) orderIndex() orderIndex {
	return newOrderIndex(op.sim, op.m.Session, op.OrderId)
}

type OperationRemove struct {
	Operation
}

func (o *OperationRemove) getOperation() *Operation {
	return &o.Operation
}
func (op *OperationRemove) CanAffect(what int) bool {
	return (what == OA_BOOKS || what == OA_ORDERS) && op.GetOptionId().Valid()
}
func (o *OperationRemove) GetOptionId() packet.OptionId {
	return o.Operation.getOptionId()
}
func (o *OperationRemove) GetSide() (side packet.MarketSide) {
	return o.Operation.getSide()
}
func (o *OperationRemove) GetDefaultSizeDelta() int {
	o.Operation.populate()
	errs.Check(o.origOrder != nil)
	return -o.origOrder.Size
}
func (o *OperationRemove) GetNewSize(sk SizeKind) int {
	errs.Check(sk == SizeKindDefault)
	return 0
}
func (o *OperationRemove) GetPrice() int {
	o.Operation.populate()
	errs.Check(o.origOrder != nil)
	return packet.PriceTo4Dec(o.origOrder.Price)
}

type OperationUpdate struct {
	Operation
	sizeChange int
}

func (o *OperationUpdate) getOperation() *Operation {
	return &o.Operation
}
func (op *OperationUpdate) CanAffect(what int) bool {
	return (what == OA_BOOKS || what == OA_ORDERS) && op.GetOptionId().Valid()
}
func (o *OperationUpdate) GetOptionId() packet.OptionId {
	return o.Operation.getOptionId()
}
func (o *OperationUpdate) GetSide() (side packet.MarketSide) {
	return o.Operation.getSide()
}
func (o *OperationUpdate) GetDefaultSizeDelta() int {
	return -o.sizeChange
}
func (o *OperationUpdate) GetNewSize(sk SizeKind) int {
	errs.Check(sk == SizeKindDefault)
	o.Operation.populate()
	errs.Check(o.origOrder != nil)
	return o.origOrder.Size - o.sizeChange
}
func (o *OperationUpdate) GetPrice() int {
	o.Operation.populate()
	errs.Check(o.origOrder != nil)
	return packet.PriceTo4Dec(o.origOrder.Price)
}

type OperationTop struct {
	Operation
	optionId packet.OptionId
	side     packet.MarketSide
	sizes    [SizeKinds]int
	price    packet.Price
}

func (o *OperationTop) getOperation() *Operation {
	return &o.Operation
}
func (op *OperationTop) CanAffect(what int) bool {
	return what == OA_BOOKS && op.GetOptionId().Valid()
}
func (o *OperationTop) GetOptionId() packet.OptionId {
	return o.optionId
}
func (o *OperationTop) GetSide() (side packet.MarketSide) {
	return o.side
}
func (o *OperationTop) GetDefaultSizeDelta() int {
	errs.Check(false)
	return 0
}
func (o *OperationTop) GetNewSize(sk SizeKind) int {
	return o.sizes[sk]
}
func (o *OperationTop) GetPrice() int {
	return packet.PriceTo4Dec(o.price)
}

type OperationScale struct {
	Operation
	optionId   packet.OptionId
	priceScale int
}

func (o *OperationScale) getOperation() *Operation {
	return &o.Operation
}
func (op *OperationScale) CanAffect(what int) bool {
	return what == OA_OPTIONS && op.GetOptionId().Valid()
}
func (o *OperationScale) GetOptionId() packet.OptionId {
	return o.optionId
}
func (o *OperationScale) GetSide() (side packet.MarketSide) {
	return packet.MarketSideUnknown
}
func (o *OperationScale) GetDefaultSizeDelta() int {
	errs.Check(false)
	return 0
}
func (o *OperationScale) GetNewSize(sk SizeKind) int {
	errs.Check(false)
	return 0
}
func (o *OperationScale) GetPrice() int {
	return o.priceScale
}
