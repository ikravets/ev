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
	//AddMessage(*IttoDbMessage)
	FindOptionId(gopacket.Flow, itto.RefNumDelta) (itto.OptionId, error)
	Stats() IttoDbStats
	MessageOperations(*IttoDbMessage) []IttoDbOperation
}

func NewIttoDb() IttoDb {
	return &db{
		order2option: make(map[uint64]itto.OptionId),
		orderSize:    make(map[uint64]int),
	}
}

type db struct {
	sessions     []session
	order2option map[uint64]itto.OptionId
	orderSize    map[uint64]int
}

type session struct {
	flow  gopacket.Flow
	index int
}

func (d *db) FindOptionId(flow gopacket.Flow, refNumD itto.RefNumDelta) (oid itto.OptionId, err error) {
	var ok bool
	if oid, ok = d.order2option[d.getOrderIndex(flow, refNumD)]; !ok {
		err = errors.New("unknown option id")
	}
	return
}

func (d *db) addOrder(flow gopacket.Flow, refNumD itto.RefNumDelta, size int, oid itto.OptionId) {
	idx := d.getOrderIndex(flow, refNumD)
	d.order2option[idx] = oid
	d.orderSize[idx] = size
}
func (d *db) removeOrder(flow gopacket.Flow, refNumD itto.RefNumDelta) {
	idx := d.getOrderIndex(flow, refNumD)
	delete(d.order2option, idx)
	delete(d.orderSize, idx)
}
func (d *db) subtractOrder(flow gopacket.Flow, refNumD itto.RefNumDelta, size int) {
	idx := d.getOrderIndex(flow, refNumD)
	if s, ok := d.orderSize[idx]; ok {
		s -= size
		if s > 0 {
			d.orderSize[idx] = s
		} else if s == 0 {
			d.removeOrder(flow, refNumD)
		} else {
			log.Fatal("negative size after update", refNumD, size, s)
		}
	}
}

func (d *db) getOrderIndex(flow gopacket.Flow, refNumD itto.RefNumDelta) uint64 {
	s := d.getSession(flow)
	return uint64(s.index)<<32 + uint64(refNumD.Delta())
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

/*
func (d *db) AddMessage(m *IttoDbMessage) {
	flow := m.Pam.Flow()
	switch im := m.Pam.Layer().(type) {
	case *itto.IttoMessageAddOrder:
		d.addOrder(flow, im.RefNumD, im.OId)
	case *itto.IttoMessageAddQuote:
		d.addOrder(flow, im.Bid.RefNumD, im.OId)
		d.addOrder(flow, im.Ask.RefNumD, im.OId)
	case *itto.IttoMessageSingleSideReplace:
		if oid, err := d.FindOptionId(flow, im.OrigRefNumD); err == nil {
			d.addOrder(flow, im.RefNumD, oid)
		}
	case *itto.IttoMessageQuoteReplace:
		oid1, err1 := d.FindOptionId(flow, im.Bid.OrigRefNumD)
		oid2, err2 := d.FindOptionId(flow, im.Ask.OrigRefNumD)
		if err1 == nil && err2 == nil && oid1 != oid2 {
			log.Fatal("quote sides has different option id")
		} else if err1 != nil || err2 != nil {
			if err1 == nil {
				oid1 = oid2
			}
			d.addOrder(flow, im.Bid.RefNumD, oid1)
			d.addOrder(flow, im.Ask.RefNumD, oid1)
		}
	}
}
*/

func (d *db) Stats() IttoDbStats {
	s := IttoDbStats{
		numOrders:   len(d.order2option),
		numSessions: len(d.sessions),
	}
	return s
}

type IttoDbOperation interface {
	GetOptionId() itto.OptionId
	Apply()
}

type Operation struct {
	m           *IttoDbMessage
	d           *db
	hitOptionId itto.OptionId
}

type OperationAdd struct {
	Operation
	optionId itto.OptionId
	itto.OrderSide
	sibling IttoDbOperation
}

func (o *OperationAdd) GetOptionId() itto.OptionId {
	if o.hitOptionId.Invalid() {
		if o.optionId.Invalid() {
			o.hitOptionId = o.sibling.GetOptionId()
		} else {
			o.hitOptionId = o.optionId
		}
	}
	return o.hitOptionId
}
func (o *OperationAdd) Apply() {
	oid := o.GetOptionId()
	if oid.Valid() {
		o.d.addOrder(o.m.Pam.Flow(), o.RefNumD, o.Size, oid)
	}
}

type OperationRemove struct {
	Operation
	origRefNumD itto.RefNumDelta
}

func (o *OperationRemove) GetOptionId() itto.OptionId {
	if o.hitOptionId.Invalid() {
		o.hitOptionId, _ = o.d.FindOptionId(o.m.Pam.Flow(), o.origRefNumD)
		/*
			var err error
			o.hitOptionId, err = o.d.FindOptionId(o.m.Pam.Flow(), o.origRefNumD)
			if err != nil {
				log.Println("OperationRemove.GetOptionId() no option for", o.origRefNumD)
			}
		*/
	}
	return o.hitOptionId
}
func (o *OperationRemove) Apply() {
	o.d.removeOrder(o.m.Pam.Flow(), o.origRefNumD)
}

type OperationUpdate struct {
	Operation
	origRefNumD itto.RefNumDelta
	sizeChange  int
}

func (o *OperationUpdate) GetOptionId() itto.OptionId {
	if o.hitOptionId.Invalid() {
		o.hitOptionId, _ = o.d.FindOptionId(o.m.Pam.Flow(), o.origRefNumD)
	}
	return o.hitOptionId
}
func (o *OperationUpdate) Apply() {
	o.d.subtractOrder(o.m.Pam.Flow(), o.origRefNumD, o.sizeChange)
}

func (d *db) MessageOperations(m *IttoDbMessage) []IttoDbOperation {
	baseOp := Operation{
		m: m,
		d: d,
	}
	var ops []IttoDbOperation
	switch im := m.Pam.Layer().(type) {
	case *itto.IttoMessageAddOrder:
		ops = append(ops, &OperationAdd{
			Operation: baseOp,
			optionId:  im.OId,
			OrderSide: im.OrderSide,
		})
	case *itto.IttoMessageAddQuote:
		ops = append(ops, &OperationAdd{
			Operation: baseOp,
			optionId:  im.OId,
			OrderSide: im.Bid,
		})
		ops = append(ops, &OperationAdd{
			Operation: baseOp,
			optionId:  im.OId,
			OrderSide: im.Ask,
		})
	case *itto.IttoMessageSingleSideExecuted:
		ops = append(ops, &OperationUpdate{
			Operation:   baseOp,
			origRefNumD: im.OrigRefNumD,
			sizeChange:  im.Size,
		})
	case *itto.IttoMessageSingleSideExecutedWithPrice:
		ops = append(ops, &OperationUpdate{
			Operation:   baseOp,
			origRefNumD: im.OrigRefNumD,
			sizeChange:  im.Size,
		})
	case *itto.IttoMessageOrderCancel:
		ops = append(ops, &OperationUpdate{
			Operation:   baseOp,
			origRefNumD: im.OrigRefNumD,
			sizeChange:  im.Size,
		})
	case *itto.IttoMessageSingleSideReplace:
		ops = append(ops, &OperationRemove{
			Operation:   baseOp,
			origRefNumD: im.OrigRefNumD,
		})
		ops = append(ops, &OperationAdd{
			Operation: baseOp,
			OrderSide: im.OrderSide,
			sibling:   ops[len(ops)-1],
		})
	case *itto.IttoMessageSingleSideDelete:
		ops = append(ops, &OperationRemove{
			Operation:   baseOp,
			origRefNumD: im.OrigRefNumD,
		})
	case *itto.IttoMessageSingleSideUpdate:
		ops = append(ops, &OperationRemove{
			Operation:   baseOp,
			origRefNumD: im.RefNumD,
		})
		ops = append(ops, &OperationAdd{
			Operation: baseOp,
			OrderSide: im.OrderSide,
			sibling:   ops[len(ops)-1],
		})
	case *itto.IttoMessageQuoteReplace:
		ops = append(ops, &OperationRemove{
			Operation:   baseOp,
			origRefNumD: im.Bid.OrigRefNumD,
		})
		ops = append(ops, &OperationAdd{
			Operation: baseOp,
			OrderSide: im.Bid.OrderSide,
			sibling:   ops[len(ops)-1],
		})
		ops = append(ops, &OperationRemove{
			Operation:   baseOp,
			origRefNumD: im.Ask.OrigRefNumD,
		})
		ops = append(ops, &OperationAdd{
			Operation: baseOp,
			OrderSide: im.Ask.OrderSide,
			sibling:   ops[len(ops)-1],
		})
	case *itto.IttoMessageQuoteDelete:
		ops = append(ops, &OperationRemove{
			Operation:   baseOp,
			origRefNumD: im.BidOrigRefNumD,
		})
		ops = append(ops, &OperationRemove{
			Operation:   baseOp,
			origRefNumD: im.AskOrigRefNumD,
		})
	case *itto.IttoMessageBlockSingleSideDelete:
		for _, r := range im.RefNumDs {
			ops = append(ops, &OperationRemove{
				Operation:   baseOp,
				origRefNumD: r,
			})
		}
	}
	return ops
}
