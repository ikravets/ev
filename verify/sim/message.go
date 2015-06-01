// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"my/itto/verify/packet"
	"my/itto/verify/packet/itto"
)

type SimMessage struct {
	Pam     packet.ApplicationMessage
	Session *Session
	sim     Sim
}

func NewSimMessage(sim Sim, pam packet.ApplicationMessage) *SimMessage {
	s := sim.Session(pam.Flow())
	m := &SimMessage{
		Pam:     pam,
		Session: &s,
		sim:     sim,
	}
	return m
}

func (m *SimMessage) IgnoredBySubscriber() bool {
	if m.sim.Subscr() == nil {
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
	return oid.Valid() && !m.sim.Subscr().Subscribed(oid)
}

func (m *SimMessage) MessageOperations() []SimOperation {
	var ops []SimOperation
	addOperation := func(origRefNumD itto.RefNumDelta, operation SimOperation) {
		opop := operation.getOperation()
		opop.m = m
		opop.sim = m.sim
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
