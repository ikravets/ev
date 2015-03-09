// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"log"

	"code.google.com/p/gopacket"

	"my/itto/verify/packet/itto"
	"my/itto/verify/sim"
)

type TobLogger struct {
	lastMessage  *sim.IttoDbMessage
	lastOptionId itto.OptionId
	consumeOps   int
	curOps       int
	hasOldTob    bool
	ittoSeconds  uint32
	bid          tob
	ask          tob
	seqNum       map[gopacket.Flow]uint64
}
type tob struct {
	Check bool
	Side  itto.MarketSide
	Old   sim.PriceLevel
	New   sim.PriceLevel
}

func NewTobLogger() *TobLogger {
	l := &TobLogger{
		bid:    tob{Side: itto.MarketSideBid},
		ask:    tob{Side: itto.MarketSideAsk},
		seqNum: make(map[gopacket.Flow]uint64),
	}
	return l
}

func (l *TobLogger) MessageArrived(idm *sim.IttoDbMessage) {
	l.lastMessage = idm

	flow := l.lastMessage.Pam.Flow()
	seq := l.lastMessage.Pam.SequenceNumber()
	if prevSeq, ok := l.seqNum[flow]; ok && prevSeq+1 != seq {
		log.Printf("seqNum gap; expected %d actual %d\n", prevSeq+1, seq)
	}
	l.seqNum[flow] = seq

	l.bid.Check, l.ask.Check = false, false
	switch m := l.lastMessage.Pam.Layer().(type) {
	case
		*itto.IttoMessageAddOrder,
		*itto.IttoMessageSingleSideExecuted,
		*itto.IttoMessageSingleSideExecutedWithPrice,
		*itto.IttoMessageOrderCancel,
		*itto.IttoMessageSingleSideDelete,
		*itto.IttoMessageBlockSingleSideDelete:
		l.consumeOps = 1
	case
		*itto.IttoMessageSingleSideReplace,
		*itto.IttoMessageSingleSideUpdate:
		l.consumeOps = 2
	case
		*itto.IttoMessageAddQuote,
		*itto.IttoMessageQuoteDelete:
		l.consumeOps = 2
		l.bid.Check, l.ask.Check = true, true
	case
		*itto.IttoMessageQuoteReplace:
		l.consumeOps = 4
		l.bid.Check, l.ask.Check = true, true
	case *itto.IttoMessageSeconds:
		l.ittoSeconds = m.Second
	case *itto.IttoMessageNoii:
		// silently ignore
		return
	default:
		log.Println("wrong message type ", l.lastMessage.Pam.Layer())
		return
	}
	l.curOps = 0
	l.hasOldTob = false
}

func (*TobLogger) OperationAppliedToOrders(sim.IttoOperation) {}

func (l *TobLogger) BeforeBookUpdate(book sim.Book, operation sim.IttoOperation) {
	if l.hasOldTob {
		return
	}
	l.lastOptionId = operation.GetOptionId()
	if l.lastOptionId.Invalid() {
		return
	}
	switch operation.GetSide() {
	case itto.MarketSideBid:
		l.bid.Check = true
	case itto.MarketSideAsk:
		l.ask.Check = true
	default:
		log.Fatalln("wrong operation side")
	}
	l.bid.update(book, l.lastOptionId, TobUpdateOld)
	l.ask.update(book, l.lastOptionId, TobUpdateOld)
	l.hasOldTob = true
}

func (l *TobLogger) AfterBookUpdate(book sim.Book, operation sim.IttoOperation, tobUpdate TobUpdate) bool {
	l.curOps++
	if l.curOps < l.consumeOps {
		return false
	}
	l.curOps = 0
	l.hasOldTob = false
	if l.lastOptionId.Invalid() {
		return false
	}
	l.bid.update(book, l.lastOptionId, tobUpdate)
	l.ask.update(book, l.lastOptionId, tobUpdate)

	return l.bid.updated() || l.ask.updated()
}

type TobUpdate byte

const (
	TobUpdateOld TobUpdate = iota
	TobUpdateNew
	TobUpdateNewForce
)

func (tob *tob) update(book sim.Book, oid itto.OptionId, u TobUpdate) {
	pl := &tob.New
	if u == TobUpdateOld {
		pl = &tob.Old
	}
	*pl = sim.PriceLevel{}
	if tob.Check || u == TobUpdateNewForce {
		if pls := book.GetTop(oid, tob.Side, 1); len(pls) > 0 {
			*pl = pls[0]
		}
	}
}

func (tob *tob) updated() bool {
	return tob.Check && tob.Old != tob.New
}
