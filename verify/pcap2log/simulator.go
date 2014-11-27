// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package pcap2log

import (
	"fmt"
	"github.com/kr/pretty"
	"io"
	"log"
)

var _ = pretty.Print

type Order struct {
	id       uint
	optionId OptionId
	price    uint
	size     uint
	side     MarketSide
}

type simulator struct {
	w                io.Writer
	ops              []OrderOperation
	orders           map[uint]Order
	options          map[OptionId]*OptionState
	assumeSubscribed bool
}

func NewSimulator(w io.Writer) simulator {
	return simulator{
		w:                w,
		orders:           make(map[uint]Order),
		options:          make(map[OptionId]*OptionState),
		assumeSubscribed: true,
	}
}

func (s *simulator) addMessage(qom *QOMessage, typeChar byte) {
	if qom.typ == MessageTypeUnknown {
		return
	}
	//log.Printf("addMessage(%#v, %c)\n", qom, typeChar)
	s.outMessageNorm(qom, typeChar)
	s.addMessageOperations(qom)
	s.processOperations()
}

type OrderOp byte

const (
	OrderOpUnknown OrderOp = iota
	OrderOpAdd
	OrderOpRemove
	OrderOpUpdate
)

type OrderOperation struct {
	op       OrderOp
	orderId  uint
	optionId OptionId
	size     uint
	price    uint
	side     MarketSide
}

func (s *simulator) addMessageOperations(m *QOMessage) {
	switch m.typ {
	case MessageTypeQuoteAdd:
		s.addOp(OrderOpAdd, m.side1, m.optionId)
		s.addOp(OrderOpAdd, m.side2, m.optionId)
	case MessageTypeQuoteReplace:
		s.addOp(OrderOpRemove, m.side1, OptionIdUnknown)
		s.addOp(OrderOpAdd, m.side1, OptionIdUnknown)
		s.addOp(OrderOpRemove, m.side2, OptionIdUnknown)
		s.addOp(OrderOpAdd, m.side2, OptionIdUnknown)
	case MessageTypeQuoteDelete:
		s.addOp(OrderOpRemove, m.side1, OptionIdUnknown)
		s.addOp(OrderOpRemove, m.side2, OptionIdUnknown)
	case MessageTypeOrderAdd:
		s.addOp(OrderOpAdd, m.side1, m.optionId)
	case MessageTypeOrderExecute, MessageTypeOrderExecuteWPrice, MessageTypeOrderCancel:
		s.addOp(OrderOpUpdate, m.side1, OptionIdUnknown)
	case MessageTypeOrderUpdate:
		s.addOp(OrderOpRemove, m.side1, OptionIdUnknown)
		order := m.side1
		order.refNumDelta = order.origRefNumDelta
		s.addOp(OrderOpAdd, order, OptionIdUnknown)
	case MessageTypeOrderReplace:
		s.addOp(OrderOpRemove, m.side1, OptionIdUnknown)
		s.addOp(OrderOpAdd, m.side1, OptionIdUnknown)
	case MessageTypeOrderDelete:
		s.addOp(OrderOpRemove, m.side1, OptionIdUnknown)
	case MessageTypeBlockOrderDelete:
		for _, r := range m.bssdRefs {
			s.addOp(OrderOpRemove, OrderSide{origRefNumDelta: r}, OptionIdUnknown)
		}
	default:
		log.Fatalf("Unexpected message type %d in %+v\n", m.typ, m)
	}
}

func (s *simulator) addOp(op OrderOp, order OrderSide, optionId OptionId) {
	var o OrderOperation
	switch op {
	case OrderOpAdd:
		o = OrderOperation{
			op:       op,
			optionId: optionId,
			orderId:  order.refNumDelta,
			size:     order.size,
			price:    order.price,
			side:     order.side,
		}
	case OrderOpRemove:
		o = OrderOperation{
			op:      op,
			orderId: order.origRefNumDelta,
		}
	case OrderOpUpdate:
		o = OrderOperation{
			op:      op,
			orderId: order.origRefNumDelta,
			size:    order.size,
		}
	default:
		log.Fatal("Unexpected order operation")
	}
	s.ops = append(s.ops, o)
}

func (s *simulator) processOperations() {
	var prev struct {
		valid     bool
		nextValid bool
		optionId  OptionId
		side      MarketSide
	}
	for i := range s.ops {
		prev.valid, prev.nextValid = prev.nextValid, false
		op := &s.ops[i]
		order, orderFound := s.orders[op.orderId]
		s.outOrderLookup(op, order, orderFound)
		if op.op == OrderOpAdd {
			if orderFound {
				log.Fatalf("Order already exist when adding op=%#v order=%#v", op, order)
			}
			if op.optionId == OptionIdUnknown {
				if s.ops[i-1].op != OrderOpRemove {
					log.Fatal("Unexpected add operation", op)
				}
				if !prev.valid || i == 0 {
					continue
				}
				op.optionId = prev.optionId
				if op.side == MarketSideUnknown {
					op.side = prev.side
				}
			}
			if s.assumeSubscribed {
				s.subscibe(op.optionId)
			} else if !s.subscribed(op.optionId) {
				continue
			}
		} else {
			if !orderFound {
				continue
			}
			if op.optionId != OptionIdUnknown || op.side != MarketSideUnknown || op.price != 0 {
				log.Fatalf("unexpected operation parameters %#v\n", op)
			}
			op.optionId = order.optionId
			op.side = order.side
			op.price = order.price
			if op.op == OrderOpRemove {
				prev.nextValid = true
				prev.optionId = order.optionId
				prev.side = order.side
			}
		}
		s.updateOrders(op, order)
		s.updateOptionState(*op)
	}
	s.ops = s.ops[:0]
}

func (s *simulator) updateOrders(op *OrderOperation, order Order) {
	switch op.op {
	case OrderOpAdd:
		order = Order{
			id:       op.orderId,
			optionId: op.optionId,
			price:    op.price,
			size:     op.size,
			side:     op.side,
		}
		s.orders[op.orderId] = order
	case OrderOpUpdate:
		order.size -= op.size
		s.orders[op.orderId] = order
	case OrderOpRemove:
		order = Order{
			id: op.orderId,
		}
		delete(s.orders, op.orderId)
	default:
		log.Fatal("Unexpected order operation", op)
	}
	s.outOrderUpdate(op, order)
}

func (s *simulator) updateOptionState(op OrderOperation) {
	if op.optionId == OptionIdUnknown || op.side == MarketSideUnknown {
		log.Fatalf("unexpected operation parameters op=%#v order=%#v newOrder=%#v\n", op)
	}
	optionState := s.options[op.optionId]
	if optionState == nil {
		log.Fatal("unexpectedly not subscribed to", op.optionId)
	}
	var delta int
	if op.op == OrderOpAdd {
		delta = int(op.size)
	} else {
		delta = int(-op.size)
	}

	side := optionState.side(op.side)
	side.updateLevel(op.price, delta)
}

func (s *simulator) subscibe(optionId OptionId) {
	if !s.subscribed(optionId) {
		s.options[optionId] = NewOptionState(optionId)
	}
}

func (s *simulator) subscribed(optionId OptionId) bool {
	_, ok := s.options[optionId]
	return ok
}

type OptionState struct {
	bid *OptionSideState
	ask *OptionSideState
}

func NewOptionState(id OptionId) *OptionState {
	return &OptionState{
		bid: NewOptionSideState(MarketSideBuy),
		ask: NewOptionSideState(MarketSideSell),
	}
}

func (s *OptionState) side(ms MarketSide) (res *OptionSideState) {
	switch ms {
	case MarketSideBuy:
		res = s.bid
	case MarketSideSell:
		res = s.ask
	default:
		log.Fatal("Unexpected market side", ms, "for option", s)
	}
	return
}

type OptionSideState struct {
	levels map[uint]uint
}

func NewOptionSideState(side MarketSide) *OptionSideState {
	s := OptionSideState{
		levels: make(map[uint]uint),
	}
	return &s
}

func (s *OptionSideState) updateLevel(price uint, delta int) {
	ssize := int(s.levels[price]) + delta
	if ssize < 0 {
		log.Fatal("size become negative")
	}
	if ssize == 0 {
		delete(s.levels, price)
	} else {
		s.levels[price] = uint(ssize)
	}
}

// output functions

func (s *simulator) Printf(format string, vs ...interface{}) {
	if _, err := fmt.Fprintf(s.w, format, vs...); err != nil {
		log.Fatal("output error", err)
	}
}

func (s *simulator) Printfln(format string, vs ...interface{}) {
	s.Printf(format, vs...)
	if _, err := fmt.Fprintln(s.w); err != nil {
		log.Fatal("output error", err)
	}
}

func (s *simulator) outMessageNorm(m *QOMessage, typeChar byte) {
	out := func(name, f string, vs ...interface{}) {
		s.Printf("NORM %s %c ", name, typeChar)
		s.Printfln(f, vs...)
	}
	ord, bid, ask := &m.side1, &m.side1, &m.side2
	if bid.side == MarketSideSell {
		bid, ask = ask, bid
	}
	switch m.typ {
	case MessageTypeQuoteAdd:
		out("QBID", "%08x %08x %08x %08x", m.optionId, bid.refNumDelta, bid.size, bid.price)
		out("QASK", "%08x %08x %08x %08x", m.optionId, ask.refNumDelta, ask.size, ask.price)
	case MessageTypeQuoteReplace:
		out("QBID", "%08x %08x %08x %08x", bid.refNumDelta, bid.origRefNumDelta, bid.size, bid.price)
		out("QASK", "%08x %08x %08x %08x", ask.refNumDelta, ask.origRefNumDelta, ask.size, ask.price)
	case MessageTypeQuoteDelete:
		out("QBID", "%08x", bid.origRefNumDelta)
		out("QASK", "%08x", ask.origRefNumDelta)
	case MessageTypeOrderAdd:
		out("ORDER", "%c %08x %08x %08x %08x", ord.side, m.optionId, ord.refNumDelta, ord.size, ord.price)
	case MessageTypeOrderExecute, MessageTypeOrderExecuteWPrice, MessageTypeOrderCancel:
		out("ORDER", "%08x %08x", ord.origRefNumDelta, ord.size)
	case MessageTypeOrderUpdate:
		out("ORDER", "%08x %08x %08x", ord.origRefNumDelta, ord.size, ord.price)
	case MessageTypeOrderReplace:
		out("ORDER", "%08x %08x %08x %08x", ord.refNumDelta, ord.origRefNumDelta, ord.size, ord.price)
	case MessageTypeOrderDelete:
		out("ORDER", "%08x", ord.origRefNumDelta)
	case MessageTypeBlockOrderDelete:
		for _, r := range m.bssdRefs {
			out("ORDER", "%08x", r)
		}
	default:
		log.Fatalf("Unexpected message type %d in %+v\n", m.typ, m)
	}
}

func bool2int(b bool) int {
	if b {
		return 1
	} else {
		return 0
	}
}

func marketSide2int(ms MarketSide) int {
	if ms == MarketSideSell {
		return 1
	} else {
		return 0
	}
}

func (s *simulator) outOrderLookup(op *OrderOperation, order Order, isFound bool) {
	if op.op == OrderOpAdd {
		s.Printfln("ORDL 1 %08x %08x", op.orderId, op.optionId)
		order.optionId = op.optionId
	} else {
		s.Printfln("ORDL 0 %08x", op.orderId)
	}
	s.Printfln("ORDRESP %d %d %d %08x %08x %08x",
		bool2int(!isFound && op.op != OrderOpAdd),
		bool2int(op.op == OrderOpAdd),
		marketSide2int(order.side),
		order.size,
		order.price,
		order.optionId,
	)

}

func (s *simulator) outOrderUpdate(op *OrderOperation, order Order) {
	s.Printfln("ORDU %08x %08x %d %08x %08x",
		order.id,
		order.optionId,
		marketSide2int(order.side),
		order.price,
		order.size,
	)
}
