// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"errors"
	"log"

	"code.google.com/p/gopacket"

	"my/itto/verify/packet"
	"my/itto/verify/packet/itto"
)

var _ = log.Ldate

type IttoDbStats struct {
	Orders     int
	PeakOrders int
	Sessions   int
}

type IttoDbMessage struct {
	Pam     packet.ApplicationMessage
	Session *Session
}

type IttoDb interface {
	Stats() IttoDbStats
	NewMessage(packet.ApplicationMessage) *IttoDbMessage
	MessageOperations(*IttoDbMessage) []IttoOperation
	ApplyOperation(operation IttoOperation)
}

func NewIttoDb() IttoDb {
	return &db{
		orders: make(map[orderIndex]order),
	}
}

type dbStatSupport struct {
	maxOrders int
}
type db struct {
	sessions []Session
	orders   map[orderIndex]order
	stat     dbStatSupport
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

type Session struct {
	flow  gopacket.Flow
	index int
}

func (s *Session) Index() int {
	return s.index
}

var orderNotFoundError = errors.New("order not found")

func (d *db) findOrder(flow gopacket.Flow, refNumD itto.RefNumDelta) (order order, err error) {
	order, ok := d.orders[NewOrderIndex(d, flow, refNumD)]
	if !ok {
		err = orderNotFoundError
	}
	return
}

func (d *db) getSession(flow gopacket.Flow) Session {
	for _, s := range d.sessions {
		if s.flow == flow {
			return s
		}
	}
	s := Session{
		flow:  flow,
		index: len(d.sessions),
	}
	d.sessions = append(d.sessions, s)
	return s
}

func (d *db) Stats() IttoDbStats {
	s := IttoDbStats{
		Orders:     len(d.orders),
		PeakOrders: d.stat.maxOrders,
		Sessions:   len(d.sessions),
	}
	return s
}

func (d *db) NewMessage(pam packet.ApplicationMessage) *IttoDbMessage {
	s := d.getSession(pam.Flow())
	m := &IttoDbMessage{
		Pam:     pam,
		Session: &s,
	}
	return m
}

func (d *db) ApplyOperation(operation IttoOperation) {
	operation.getOperation().populate()
	oid := operation.GetOptionId()
	if oid.Invalid() {
		return
	}
	switch op := operation.(type) {
	case *OperationAdd:
		// intentionally allow adding zero price/size orders
		o := order{OId: op.optionId, OrderSide: op.OrderSide}
		if op.origOrder != nil {
			if op.optionId.Valid() {
				log.Fatalf("bad option id for add operation %#v origOrder=%#v\n", op, *op.origOrder)
			}
			if op.Side != itto.MarketSideUnknown && op.Side != op.origOrder.Side {
				log.Fatalf("bad side for add operation %#v origOrder=%#v\n", op, *op.origOrder)
			}
			o.OId = op.origOrder.OId
			o.Side = op.origOrder.Side
		}
		d.orders[op.orderIndex()] = o
		if l := len(d.orders); l > d.stat.maxOrders {
			d.stat.maxOrders = l
		}
	default:
		o := *operation.getOperation().origOrder
		oidx := operation.getOperation().origOrderIndex()
		o.Size += op.GetSizeDelta()
		switch {
		case o.Size > 0:
			d.orders[oidx] = o
		case o.Size == 0:
			// treat OperationUpdate which zeroes order size as order removal
			delete(d.orders, oidx)
		case o.Size < 0:
			log.Fatalf("negative size after operation %#v origOrder=%#v\n", operation, o)
		}
	}
}

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
