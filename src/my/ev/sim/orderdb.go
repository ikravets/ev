// Copyright (c) Ilia Kravets, 2014-2016. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"errors"
	"log"

	"github.com/ikravets/errs"

	"my/ev/packet"
)

type OrderDb interface {
	Stats() OrderDbStats
	ApplyOperation(operation SimOperation)
	findOrder(session *Session, orderId packet.OrderId) (order order, err error)
}
type OrderDbStats struct {
	Orders     int
	PeakOrders int
}

type orderDb struct {
	sim    Sim
	orders map[orderIndex]order
	stat   dbStatSupport
}
type dbStatSupport struct {
	maxOrders int
}

func NewOrderDb(sim Sim) OrderDb {
	return &orderDb{
		sim:    sim,
		orders: make(map[orderIndex]order),
	}
}

type orderIndex struct {
	orderId      packet.OrderId
	sessionIndex int
}

func newOrderIndex(sim Sim, session *Session, orderId packet.OrderId) orderIndex {
	return orderIndex{orderId: orderId, sessionIndex: session.index}
}

type order struct {
	OptionId packet.OptionId
	OrderId  packet.OrderId
	Side     packet.MarketSide
	Price    packet.Price
	Size     int
}

var orderNotFoundError = errors.New("order not found")

func (d *orderDb) findOrder(session *Session, orderId packet.OrderId) (order order, err error) {
	order, ok := d.orders[newOrderIndex(d.sim, session, orderId)]
	if !ok {
		err = orderNotFoundError
	}
	return
}

func (d *orderDb) ApplyOperation(operation SimOperation) {
	switch op := operation.(type) {
	case *OperationAdd:
		// intentionally allow adding zero price/size orders
		o := op.order
		if op.origOrder != nil {
			if op.OptionId.Valid() {
				log.Fatalf("bad option id for add operation %#v origOrder=%#v\n", op, *op.origOrder)
			}
			if op.Side != packet.MarketSideUnknown && op.Side != op.origOrder.Side {
				log.Fatalf("bad side for add operation %#v origOrder=%#v\n", op, *op.origOrder)
			}
			o.OptionId = op.origOrder.OptionId
			o.Side = op.origOrder.Side
		}
		d.orders[op.orderIndex()] = o
		if l := len(d.orders); l > d.stat.maxOrders {
			d.stat.maxOrders = l
		}
	case *OperationRemove, *OperationUpdate:
		o := *operation.getOperation().origOrder
		oidx := operation.getOperation().origOrderIndex()
		o.Size += op.GetDefaultSizeDelta()
		switch {
		case o.Size > 0:
			d.orders[oidx] = o
		case o.Size == 0:
			// treat OperationUpdate which zeroes order size as order removal
			delete(d.orders, oidx)
		case o.Size < 0:
			log.Fatalf("negative size after operation %#v origOrder=%#v\n", operation, o)
		}
	default:
		errs.Check(false)
	}
}

func (d *orderDb) Stats() OrderDbStats {
	s := OrderDbStats{
		Orders:     len(d.orders),
		PeakOrders: d.stat.maxOrders,
	}
	return s
}
