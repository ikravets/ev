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
	w      io.Writer
	ops    []OrderOperation
	orders map[uint]Order
}

func NewSimulator(w io.Writer) simulator {
	return simulator{
		w:      w,
		orders: make(map[uint]Order),
	}
}

func (s *simulator) addMessage(qom *QOMessage, typeChar byte) {
	if qom.typ == MessageTypeUnknown {
		return
	}
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
		s.addOp(OrderOpAdd, m.side1, OptionIdUnknown)
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
	var lastOrder Order
	for i := range s.ops {
		op := &s.ops[i]
		order, orderFound := s.orders[op.orderId]
		s.outOrderLookup(op, order, orderFound)
		if orderFound && op.op == OrderOpAdd {
			log.Fatal("Order already exist when adding", op, order)
		}
		if !orderFound && op.op != OrderOpAdd {
			continue
		}
		if op.op == OrderOpAdd && op.optionId == OptionIdUnknown {
			if i < 1 || s.ops[i-1].op != OrderOpRemove {
				log.Fatal("Unexpected add operation", op)
			}
			op.optionId = lastOrder.optionId
		}
		lastOrder = order
		order = s.processOperation(op)
		s.outOrderUpdate(op, order)
	}
	s.ops = s.ops[:0]
}

func (s *simulator) processOperation(op *OrderOperation) Order {
	var order Order
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
		order = s.orders[op.orderId]
		order.size -= op.size
		s.orders[op.orderId] = order
	case OrderOpRemove:
		order.id = op.orderId
		delete(s.orders, op.orderId)
	default:
		log.Fatal("Unexpected order operation", op)
	}
	return order
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
	switch ms {
	case MarketSideSell:
		return 1
	case MarketSideBuy:
		return 0
	default:
		return -1
	}
}

func (s *simulator) outOrderLookup(op *OrderOperation, order Order, isFound bool) {
	if op.op == OrderOpAdd {
		s.Printfln("ORDL 1 %08x %08x", op.orderId, op.optionId)
		order.optionId = op.optionId
		order.side = MarketSideBuy
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
