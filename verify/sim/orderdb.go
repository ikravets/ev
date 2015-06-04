// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"errors"
	"log"

	"code.google.com/p/gopacket"

	"my/itto/verify/packet/itto"
)

type OrderDb interface {
	Stats() OrderDbStats
	ApplyOperation(operation SimOperation)
	findOrder(flow gopacket.Flow, refNumD itto.RefNumDelta) (order order, err error) // TODO refactor
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

type orderIndex uint64

func newOrderIndex(sim Sim, flow gopacket.Flow, refNumD itto.RefNumDelta) orderIndex {
	s := sim.Session(flow)
	return orderIndex(uint64(s.index)<<32 + uint64(refNumD.Delta()))
}

type order struct {
	OId itto.OptionId
	itto.OrderSide
}

var orderNotFoundError = errors.New("order not found")

func (d *orderDb) findOrder(flow gopacket.Flow, refNumD itto.RefNumDelta) (order order, err error) {
	order, ok := d.orders[newOrderIndex(d.sim, flow, refNumD)]
	if !ok {
		err = orderNotFoundError
	}
	return
}

func (d *orderDb) ApplyOperation(operation SimOperation) {
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

func (d *orderDb) Stats() OrderDbStats {
	s := OrderDbStats{
		Orders:     len(d.orders),
		PeakOrders: d.stat.maxOrders,
	}
	return s
}