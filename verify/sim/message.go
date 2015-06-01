// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"my/itto/verify/packet"
	"my/itto/verify/packet/itto"
)

type IttoDbMessage struct {
	Pam     packet.ApplicationMessage
	Session *Session
	subscr  *Subscr
}

func (d *db) NewMessage(pam packet.ApplicationMessage) *IttoDbMessage {
	s := d.getSession(pam.Flow())
	m := &IttoDbMessage{
		Pam:     pam,
		Session: &s,
		subscr:  d.subscr,
	}
	return m
}

func (m *IttoDbMessage) IgnoredBySubscriber() bool {
	if m.subscr == nil {
		return false
	}
	var oid itto.OptionId
	switch im := m.Pam.Layer().(type) {
	case *itto.IttoMessageAddOrder:
		oid = im.OId
	case *itto.IttoMessageAddQuote:
		oid = im.OId
	case *itto.IttoMessageOptionsTrade:
		oid = im.OId
	case *itto.IttoMessageOptionsCrossTrade:
		oid = im.OId
	}
	return oid.Valid() && !m.subscr.Subscribed(oid)
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
		var oid itto.OptionId
		if !m.IgnoredBySubscriber() {
			oid = im.OId
		}
		addOperation(itto.RefNumDelta{}, &OperationAdd{optionId: oid, OrderSide: im.OrderSide})
	case *itto.IttoMessageAddQuote:
		var oid itto.OptionId
		if !m.IgnoredBySubscriber() {
			oid = im.OId
		}
		addOperation(itto.RefNumDelta{}, &OperationAdd{optionId: oid, OrderSide: im.Bid})
		addOperation(itto.RefNumDelta{}, &OperationAdd{optionId: oid, OrderSide: im.Ask})
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
