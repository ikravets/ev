// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"log"

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
	var oid packet.OptionId
	if m, ok := m.Pam.Layer().(packet.TradeMessage); ok {
		oid, _, _ = m.TradeInfo()
	}
	return oid.Valid() && !m.sim.Subscr().Subscribed(oid)
}

func (m *SimMessage) MessageOperations() []SimOperation {
	var ops []SimOperation
	addOperation := func(origOrderId packet.OrderId, operation SimOperation) {
		opop := operation.getOperation()
		opop.m = m
		opop.sim = m.sim
		opop.origOrderId = origOrderId
		ops = append(ops, operation)
	}
	addOperationReplace := func(origOrderId packet.OrderId, orderSide itto.OrderSide) {
		opRemove := &OperationRemove{}
		opAdd := &OperationAdd{
			// unknown: optionId; maybe unknown: OrderSide.Side
			order:     orderFromItto(packet.OptionIdUnknown, orderSide),
			Operation: Operation{sibling: opRemove},
		}
		addOperation(origOrderId, opRemove)
		addOperation(packet.OrderIdUnknown, opAdd)
	}
	switch im := m.Pam.Layer().(type) {
	case *itto.IttoMessageAddOrder:
		var oid packet.OptionId
		if !m.IgnoredBySubscriber() {
			oid = im.OId
		}
		addOperation(packet.OrderIdUnknown, &OperationAdd{order: orderFromItto(oid, im.OrderSide)})
	case *itto.IttoMessageAddQuote:
		var oid packet.OptionId
		if !m.IgnoredBySubscriber() {
			oid = im.OId
		}
		addOperation(packet.OrderIdUnknown, &OperationAdd{order: orderFromItto(oid, im.Bid)})
		addOperation(packet.OrderIdUnknown, &OperationAdd{order: orderFromItto(oid, im.Ask)})
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
	case
		*itto.IttoMessageNoii,
		*itto.IttoMessageOptionsTrade,
		*itto.IttoMessageOptionsCrossTrade,
		*itto.IttoMessageOptionDirectory,
		*itto.IttoMessageOptionOpen,
		*itto.IttoMessageOptionTradingAction:
		// silently ignore
	default:
		log.Println("unexpected message ", m.Pam.Layer())
	}
	return ops
}

func orderFromItto(oid packet.OptionId, os itto.OrderSide) order {
	return order{
		OptionId: oid,
		OrderId:  os.RefNumD,
		Side:     os.Side,
		Price:    os.Price,
		Size:     os.Size,
	}
}
