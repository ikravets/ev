// Copyright (c) Ilia Kravets, 2014-2016. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"log"

	"github.com/ikravets/errs"

	"my/ev/packet"
	"my/ev/sim"
)

type TobLogger struct {
	lastOptionId packet.OptionId
	consumeOps   int
	curOps       int
	hasOldTob    bool
	bid          tob
	ask          tob
}
type tob struct {
	Check bool
	Side  packet.MarketSide
	Old   sim.PriceLevel
	New   sim.PriceLevel
	Flags TobUpdate
}

func NewTobLogger() *TobLogger {
	l := &TobLogger{
		bid: tob{Side: packet.MarketSideBid},
		ask: tob{Side: packet.MarketSideAsk},
	}
	return l
}

func (l *TobLogger) SetUpdateFlags(flags TobUpdate) {
	l.bid.Flags = flags
	l.ask.Flags = flags
}

func (l *TobLogger) MessageArrived(idm *sim.SimMessage) {
	l.consumeOps = idm.BookUpdates()
	if idm.BookSides() == 2 {
		// XXX why do we need this?
		l.bid.Check, l.ask.Check = true, true
	} else {
		l.bid.Check, l.ask.Check = false, false
	}
	l.curOps = 0
	l.hasOldTob = false
}

func (*TobLogger) OperationAppliedToOrders(sim.SimOperation) {}

func (l *TobLogger) BeforeBookUpdate(book sim.Book, operation sim.SimOperation) {
	errs.Check(l.consumeOps != 0)
	if l.hasOldTob {
		return
	}
	l.lastOptionId = operation.GetOptionId()
	if l.lastOptionId.Invalid() {
		return
	}
	switch operation.GetSide() {
	case packet.MarketSideBid:
		l.bid.Check = true
	case packet.MarketSideAsk:
		l.ask.Check = true
	default:
		log.Fatalln("wrong operation side")
	}
	l.bid.update(book, l.lastOptionId, false)
	l.ask.update(book, l.lastOptionId, false)
	l.hasOldTob = true
}

func (l *TobLogger) AfterBookUpdate(book sim.Book, operation sim.SimOperation) bool {
	errs.Check(l.consumeOps != 0)
	l.curOps++
	if l.curOps < l.consumeOps {
		return false
	}
	l.curOps = 0
	l.hasOldTob = false
	if l.lastOptionId.Invalid() {
		return false
	}
	l.bid.update(book, l.lastOptionId, true)
	l.ask.update(book, l.lastOptionId, true)

	return l.bid.updated() || l.ask.updated()
}

type TobUpdate byte

const (
	TobUpdateBothSides TobUpdate = 1 << iota
	TobUpdateAssumeUpdated
)

func (tob *tob) update(book sim.Book, oid packet.OptionId, updateNew bool) {
	pl := &tob.Old
	if updateNew {
		pl = &tob.New
	}
	*pl = sim.EmptyPriceLevel
	if tob.Check || tob.Flags&TobUpdateBothSides != 0 && updateNew {
		if pls := book.GetTop(oid, tob.Side, 1); len(pls) > 0 {
			*pl = pls[0].Clone()
		}
	}
}

func (tob *tob) updated() bool {
	return tob.Check && (tob.Flags&TobUpdateAssumeUpdated != 0 || !tob.Old.Equals(tob.New) && (tob.Old.Price() != 0 || tob.New.Price() != 0))
}
