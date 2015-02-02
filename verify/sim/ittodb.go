// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"errors"
	"log"
	"my/itto/verify/packet"
	"my/itto/verify/packet/itto"

	"code.google.com/p/gopacket"
)

var _ = log.Ldate

type IttoDbStats struct {
	numOrders   int
	numOptions  int
	numSessions int
}

type IttoDbMessage struct {
	Pam packet.ApplicationMessage
}

type IttoDb interface {
	Stats() IttoDbStats
	MessageOperations(*IttoDbMessage) []IttoOperation
	ApplyOperation(operation IttoOperation)
}

func NewIttoDb() IttoDb {
	return &db{
		orders: make(map[orderIndex]order),
	}
}

type db struct {
	sessions []session
	orders   map[orderIndex]order
}

type orderIndex uint64

func NewOrderIndex(d *db, flow gopacket.Flow, refNumD itto.RefNumDelta) orderIndex {
	s := d.getSession(flow)
	return orderIndex(uint64(s.index)<<32 + uint64(refNumD.Delta()))
}

type order struct {
	OId itto.OptionId
	itto.OrderSide
}

type session struct {
	flow  gopacket.Flow
	index int
}

func (d *db) findOrder(flow gopacket.Flow, refNumD itto.RefNumDelta) (order order, err error) {
	order, ok := d.orders[NewOrderIndex(d, flow, refNumD)]
	if !ok {
		err = errors.New("order not found")
	}
	return
}

func (d *db) getSession(flow gopacket.Flow) session {
	for _, s := range d.sessions {
		if s.flow == flow {
			return s
		}
	}
	s := session{
		flow:  flow,
		index: len(d.sessions),
	}
	d.sessions = append(d.sessions, s)
	return s
}

func (d *db) Stats() IttoDbStats {
	s := IttoDbStats{
		numOrders:   len(d.orders),
		numSessions: len(d.sessions),
	}
	return s
}

func (d *db) ApplyOperation(operation IttoOperation) {
	operation.getOperation().populate()
	oid := operation.GetOptionId()
	if oid.Invalid() {
		return
	}
	switch op := operation.(type) {
	case *OperationAdd:
		newOrder := order{OId: op.optionId, OrderSide: op.OrderSide}
		if op.origOrder != nil {
			if op.optionId.Valid() {
				log.Fatalf("bad option id for add operation %#v origOrder=%#v\n", op, *op.origOrder)
			}
			if op.Side != itto.MarketSideUnknown && op.Side != op.origOrder.Side {
				log.Fatalf("bad side for add operation %#v origOrder=%#v\n", op, *op.origOrder)
			}
			newOrder.OId = op.origOrder.OId
			newOrder.Side = op.origOrder.Side
		}
		d.orders[op.orderIndex()] = newOrder
	case *OperationRemove:
		delete(d.orders, op.origOrderIndex())
	case *OperationUpdate:
		o := *op.origOrder
		o.Size -= op.sizeChange
		switch {
		case o.Size > 0:
			d.orders[op.origOrderIndex()] = o
		case o.Size == 0:
			delete(d.orders, op.origOrderIndex())
		case o.Size < 0:
			log.Fatalf("negative size after operation %#v origOrder=%#v\n", op, *op.origOrder)
		}
	default:
		log.Fatal("unknown operation ", operation)
	}
}

type IttoOperation interface {
	GetOptionId() itto.OptionId
	getOperation() *Operation
}

type Operation struct {
	m           *IttoDbMessage
	d           *db
	origRefNumD itto.RefNumDelta
	origOrder   *order
	sibling     IttoOperation
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

type OperationAdd struct {
	Operation
	optionId itto.OptionId
	itto.OrderSide
}

func (o *OperationAdd) getOperation() *Operation {
	return &o.Operation
}
func (o *OperationAdd) GetOptionId() itto.OptionId {
	if o.optionId.Valid() {
		return o.optionId
	} else {
		return o.Operation.getOptionId()
	}
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

func (d *db) MessageOperations(m *IttoDbMessage) []IttoOperation {
	var ops []IttoOperation
	addOperation := func(origRefNumD itto.RefNumDelta, operation IttoOperation) {
		opop := operation.getOperation()
		opop.m = m
		opop.d = d
		opop.origRefNumD = origRefNumD
		ops = append(ops, operation)
	}
	addOperationReplace := func(origRefNumD itto.RefNumDelta, orderSide itto.OrderSide) {
		opRemove := &OperationRemove{}
		opAdd := &OperationAdd{
			// unknown: optionId; maybe unknown: OrderSide.Side
			OrderSide: orderSide,
			Operation: Operation{sibling: opRemove},
		}
		addOperation(origRefNumD, opRemove)
		addOperation(itto.RefNumDelta{}, opAdd)
	}
	switch im := m.Pam.Layer().(type) {
	case *itto.IttoMessageAddOrder:
		addOperation(itto.RefNumDelta{}, &OperationAdd{optionId: im.OId, OrderSide: im.OrderSide})
	case *itto.IttoMessageAddQuote:
		addOperation(itto.RefNumDelta{}, &OperationAdd{optionId: im.OId, OrderSide: im.Bid})
		addOperation(itto.RefNumDelta{}, &OperationAdd{optionId: im.OId, OrderSide: im.Ask})
	case *itto.IttoMessageSingleSideExecuted:
		addOperation(im.OrigRefNumD, &OperationUpdate{sizeChange: im.Size})
	case *itto.IttoMessageSingleSideExecutedWithPrice:
		addOperation(im.OrigRefNumD, &OperationUpdate{sizeChange: im.Size})
	case *itto.IttoMessageOrderCancel:
		addOperation(im.OrigRefNumD, &OperationUpdate{sizeChange: im.Size})
	case *itto.IttoMessageSingleSideReplace:
		addOperationReplace(im.OrigRefNumD, im.OrderSide)
	case *itto.IttoMessageSingleSideDelete:
		addOperation(im.OrigRefNumD, &OperationRemove{})
	case *itto.IttoMessageSingleSideUpdate:
		addOperationReplace(im.RefNumD, im.OrderSide)
	case *itto.IttoMessageQuoteReplace:
		addOperationReplace(im.Bid.OrigRefNumD, im.Bid.OrderSide)
		addOperationReplace(im.Ask.OrigRefNumD, im.Ask.OrderSide)
	case *itto.IttoMessageQuoteDelete:
		addOperation(im.BidOrigRefNumD, &OperationRemove{})
		addOperation(im.AskOrigRefNumD, &OperationRemove{})
	case *itto.IttoMessageBlockSingleSideDelete:
		for _, r := range im.RefNumDs {
			addOperation(r, &OperationRemove{})
		}
	}
	return ops
}
