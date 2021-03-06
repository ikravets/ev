// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"errors"
	"log"

	"my/ev/packet"
	"my/ev/packet/bats"
	"my/ev/packet/miax"
	"my/ev/packet/nasdaq"
)

type SimMessage struct {
	Pam        packet.ApplicationMessage
	Session    *Session
	sim        Sim
	opsPerBook int
	sides      int
	ops        []SimOperation
}

func NewSimMessage(sim Sim, pam packet.ApplicationMessage) *SimMessage {
	s := sim.Session(pam.Flows())
	m := &SimMessage{
		Pam:     pam,
		Session: &s,
		sim:     sim,
	}
	m.populateOps()
	return m
}

var notTrade = errors.New("not a trade")

func (m *SimMessage) TradeInfo() (oid packet.OptionId, price packet.Price, size int, err error) {
	if tm, ok := m.Pam.Layer().(packet.TradeMessage); ok {
		oid, price, size = tm.TradeInfo()
		price = m.scalePrice(price)
	} else {
		err = notTrade
	}
	return
}
func (m *SimMessage) BookUpdates() int {
	return m.opsPerBook
}
func (m *SimMessage) BookSides() int {
	// XXX why do we need this?
	return m.sides
}
func (m *SimMessage) MessageOperations() []SimOperation {
	return m.ops
}
func (m *SimMessage) populateOps() {
	addOperation := func(origOrderId packet.OrderId, operation SimOperation) {
		opop := operation.getOperation()
		opop.m = m
		opop.sim = m.sim
		opop.origOrderId = origOrderId
		m.ops = append(m.ops, operation)
	}
	addOperationReplace := func(origOrderId packet.OrderId, ord order) {
		opRemove := &OperationRemove{}
		opAdd := &OperationAdd{
			// unknown: optionId; maybe unknown: OrderSide.Side
			order:     ord,
			Operation: Operation{sibling: opRemove},
		}
		addOperation(origOrderId, opRemove)
		addOperation(packet.OrderIdUnknown, opAdd)
	}
	addTom := func(s miax.TomSide) {
		op := OperationTop{
			optionId: m.subscribedOptionId(),
			side:     s.Side,
			price:    s.Price,
			sizes: [SizeKinds]int{
				SizeKindDefault:  s.Size,
				SizeKindCustomer: s.PriorityCustomerSize,
			},
		}
		addOperation(packet.OrderIdUnknown, &op)
	}
	switch im := m.Pam.Layer().(type) {
	case *nasdaq.IttoMessageAddOrder:
		addOperation(packet.OrderIdUnknown, &OperationAdd{order: orderFromItto(m.subscribedOptionId(), im.OrderSide)})
	case *nasdaq.IttoMessageAddQuote:
		addOperation(packet.OrderIdUnknown, &OperationAdd{order: orderFromItto(m.subscribedOptionId(), im.Bid)})
		addOperation(packet.OrderIdUnknown, &OperationAdd{order: orderFromItto(m.subscribedOptionId(), im.Ask)})
		m.sides = 2
	case *nasdaq.IttoMessageSingleSideExecuted:
		addOperation(im.OrigRefNumD, &OperationUpdate{sizeChange: im.Size})
	case *nasdaq.IttoMessageSingleSideExecutedWithPrice:
		addOperation(im.OrigRefNumD, &OperationUpdate{sizeChange: im.Size})
	case *nasdaq.IttoMessageOrderCancel:
		addOperation(im.OrigRefNumD, &OperationUpdate{sizeChange: im.Size})
	case *nasdaq.IttoMessageSingleSideReplace:
		addOperationReplace(im.OrigRefNumD, orderFromItto(packet.OptionIdUnknown, im.OrderSide))
	case *nasdaq.IttoMessageSingleSideDelete:
		addOperation(im.OrigRefNumD, &OperationRemove{})
	case *nasdaq.IttoMessageSingleSideUpdate:
		addOperationReplace(im.RefNumD, orderFromItto(packet.OptionIdUnknown, im.OrderSide))
	case *nasdaq.IttoMessageQuoteReplace:
		addOperationReplace(im.Bid.OrigRefNumD, orderFromItto(packet.OptionIdUnknown, im.Bid.OrderSide))
		addOperationReplace(im.Ask.OrigRefNumD, orderFromItto(packet.OptionIdUnknown, im.Ask.OrderSide))
		m.sides = 2
	case *nasdaq.IttoMessageQuoteDelete:
		addOperation(im.BidOrigRefNumD, &OperationRemove{})
		addOperation(im.AskOrigRefNumD, &OperationRemove{})
		m.sides = 2
	case *nasdaq.IttoMessageBlockSingleSideDelete:
		m.opsPerBook = 1
		for _, r := range im.RefNumDs {
			addOperation(r, &OperationRemove{})
		}
	case *bats.PitchMessageAddOrder:
		ord := order{
			OptionId: m.subscribedOptionId(),
			OrderId:  im.OrderId,
			Side:     im.Side,
			Price:    im.Price,
			Size:     int(im.Size),
		}
		addOperation(packet.OrderIdUnknown, &OperationAdd{order: ord})
	case *bats.PitchMessageDeleteOrder:
		addOperation(im.OrderId, &OperationRemove{})
	case *bats.PitchMessageOrderExecuted:
		addOperation(im.OrderId, &OperationUpdate{sizeChange: int(im.Size)})
	case *bats.PitchMessageOrderExecutedAtPriceSize:
		addOperation(im.OrderId, &OperationUpdate{sizeChange: int(im.Size)})
	case *bats.PitchMessageReduceSize:
		addOperation(im.OrderId, &OperationUpdate{sizeChange: int(im.Size)})
	case *bats.PitchMessageModifyOrder:
		ord := order{
			OrderId: im.OrderId,
			Price:   im.Price,
			Size:    int(im.Size),
		}
		addOperationReplace(im.OrderId, ord)
	case *miax.TomMessageTom:
		addTom(im.TomSide)
	case *miax.TomMessageQuote:
		addTom(im.Bid)
		addTom(im.Ask)
		m.sides = 2
	case
		*nasdaq.IttoMessageNoii,
		*nasdaq.IttoMessageOptionsTrade,
		*nasdaq.IttoMessageOptionsCrossTrade,
		*nasdaq.IttoMessageOptionDirectory,
		*nasdaq.IttoMessageOptionOpen,
		*nasdaq.IttoMessageOptionTradingAction,
		*nasdaq.IttoMessageSeconds,
		*bats.PitchMessageTime,
		*bats.PitchMessageSymbolMapping,
		*bats.PitchMessageTrade,
		*bats.PitchMessageTradingStatus,
		*miax.TomMessageLiquiditySeeking,
		*miax.TomMessageTrade,
		*miax.TomMessageSeriesUpdate,
		*miax.TomMessageUnderlyingTradeStatus,
		*miax.TomMessageSystemTime,
		*miax.TomMessageUnknown: // FIXME
		// silently ignore
	default:
		log.Printf("unexpected message %#v\n", m.Pam.Layer())
	}
	if m.opsPerBook == 0 {
		m.opsPerBook = len(m.ops)
	}
	if m.sides == 0 && len(m.ops) > 0 {
		m.sides = 1
	}
}
func (m *SimMessage) subscribedOptionId() packet.OptionId {
	em := m.Pam.Layer().(packet.ExchangeMessage)
	if m.sim.Subscr() == nil || m.sim.Subscr().Subscribed(em.OptionId()) {
		return em.OptionId()
	}
	return packet.OptionIdUnknown
}
func (m *SimMessage) scalePrice(price packet.Price) packet.Price {
	if m.sim.Options() == nil {
		return price
	}
	em := m.Pam.Layer().(packet.ExchangeMessage)
	return price.Scale(m.sim.Options().PriceScale(em.OptionId()))
}

func orderFromItto(oid packet.OptionId, os nasdaq.OrderSide) order {
	return order{
		OptionId: oid,
		OrderId:  os.RefNumD,
		Side:     os.Side,
		Price:    os.Price,
		Size:     os.Size,
	}
}
