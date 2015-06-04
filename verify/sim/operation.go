// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"log"

	"my/itto/verify/packet"
)

type SimOperation interface {
	GetMessage() *SimMessage
	GetOptionId() packet.OptionId
	GetOrigOrderId() packet.OrderId
	GetSide() packet.MarketSide
	GetSizeDelta() int
	GetNewSize() int
	GetPrice() int
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
		if ord, err := op.sim.OrderDb().findOrder(op.m.Pam.Flow(), op.origOrderId); err == nil {
			op.origOrder = &ord
		}
	}
}
func (op *Operation) origOrderIndex() orderIndex {
	return newOrderIndex(op.sim, op.m.Pam.Flow(), op.origOrderId)
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
func (o *OperationAdd) GetSizeDelta() int {
	return o.Size
}
func (o *OperationAdd) GetNewSize() int {
	return o.Size
}
func (op *OperationAdd) orderIndex() orderIndex {
	return newOrderIndex(op.sim, op.m.Pam.Flow(), op.OrderId)
}

type OperationRemove struct {
	Operation
}

func (o *OperationRemove) getOperation() *Operation {
	return &o.Operation
}
func (o *OperationRemove) GetOptionId() packet.OptionId {
	return o.Operation.getOptionId()
}
func (o *OperationRemove) GetSide() (side packet.MarketSide) {
	return o.Operation.getSide()
}
func (o *OperationRemove) GetSizeDelta() int {
	o.Operation.populate()
	if o.origOrder == nil {
		log.Fatal("no origOrder")
	}
	return -o.origOrder.Size
}
func (o *OperationRemove) GetNewSize() int {
	return 0
}
func (o *OperationRemove) GetPrice() int {
	o.Operation.populate()
	if o.origOrder == nil {
		log.Fatal("no origOrder")
	}
	return packet.PriceTo4Dec(o.origOrder.Price)
}

type OperationUpdate struct {
	Operation
	sizeChange int
}

func (o *OperationUpdate) getOperation() *Operation {
	return &o.Operation
}
func (o *OperationUpdate) GetOptionId() packet.OptionId {
	return o.Operation.getOptionId()
}
func (o *OperationUpdate) GetSide() (side packet.MarketSide) {
	return o.Operation.getSide()
}
func (o *OperationUpdate) GetSizeDelta() int {
	return -o.sizeChange
}
func (o *OperationUpdate) GetNewSize() int {
	o.Operation.populate()
	if o.origOrder == nil {
		log.Fatal("no origOrder")
	}
	return o.origOrder.Size - o.sizeChange
}
func (o *OperationUpdate) GetPrice() int {
	o.Operation.populate()
	if o.origOrder == nil {
		log.Fatal("no origOrder")
	}
	return packet.PriceTo4Dec(o.origOrder.Price)
}
