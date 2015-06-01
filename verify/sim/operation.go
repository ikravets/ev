// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"log"

	"my/itto/verify/packet/itto"
)

type IttoOperation interface {
	GetMessage() *IttoDbMessage
	GetOptionId() itto.OptionId
	GetOrigRef() itto.RefNumDelta
	GetSide() itto.MarketSide
	GetSizeDelta() int
	GetNewSize() int
	GetPrice() int
	getOperation() *Operation
}

type Operation struct {
	m           *IttoDbMessage
	d           *db
	origRefNumD itto.RefNumDelta
	origOrder   *order
	sibling     IttoOperation
}

func (op *Operation) GetMessage() *IttoDbMessage {
	return op.m
}
func (op *Operation) GetOrigRef() itto.RefNumDelta {
	return op.origRefNumD
}
func (op *Operation) populate() {
	if op.origOrder != nil {
		return
	}
	if op.sibling != nil {
		op.sibling.getOperation().populate()
		op.origOrder = op.sibling.getOperation().origOrder
	} else if op.origRefNumD != (itto.RefNumDelta{}) {
		if ord, err := op.d.findOrder(op.m.Pam.Flow(), op.origRefNumD); err == nil {
			op.origOrder = &ord
		}
	}
}
func (op *Operation) origOrderIndex() orderIndex {
	return NewOrderIndex(op.d, op.m.Pam.Flow(), op.origRefNumD)
}
func (o *Operation) getOptionId() (oid itto.OptionId) {
	o.populate()
	if o.origOrder != nil {
		return o.origOrder.OId
	} else {
		return itto.OptionId(0)
	}
}
func (o *Operation) getSide() (side itto.MarketSide) {
	o.populate()
	if o.origOrder != nil {
		side = o.origOrder.Side
	}
	return
}

type OperationAdd struct {
	Operation
	optionId itto.OptionId
	itto.OrderSide
}

func (o *OperationAdd) getOperation() *Operation {
	return &o.Operation
}
func (o *OperationAdd) Independent() bool {
	return o.optionId.Valid()
}
func (o *OperationAdd) GetOptionId() itto.OptionId {
	if o.optionId.Valid() {
		return o.optionId
	} else {
		return o.Operation.getOptionId()
	}
}
func (o *OperationAdd) GetSide() (side itto.MarketSide) {
	if o.Side != itto.MarketSideUnknown {
		return o.Side
	} else {
		return o.Operation.getSide()
	}
}
func (o *OperationAdd) GetPrice() int {
	return o.Price
}
func (o *OperationAdd) GetSizeDelta() int {
	return o.Size
}
func (o *OperationAdd) GetNewSize() int {
	return o.Size
}
func (op *OperationAdd) orderIndex() orderIndex {
	return NewOrderIndex(op.d, op.m.Pam.Flow(), op.RefNumD)
}

type OperationRemove struct {
	Operation
}

func (o *OperationRemove) getOperation() *Operation {
	return &o.Operation
}
func (o *OperationRemove) GetOptionId() itto.OptionId {
	return o.Operation.getOptionId()
}
func (o *OperationRemove) GetSide() (side itto.MarketSide) {
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
	return o.origOrder.Price
}

type OperationUpdate struct {
	Operation
	sizeChange int
}

func (o *OperationUpdate) getOperation() *Operation {
	return &o.Operation
}
func (o *OperationUpdate) GetOptionId() itto.OptionId {
	return o.Operation.getOptionId()
}
func (o *OperationUpdate) GetSide() (side itto.MarketSide) {
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
	return o.origOrder.Price
}
