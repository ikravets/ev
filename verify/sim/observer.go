// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"fmt"
	"io"
	"log"

	"my/itto/verify/packet/itto"
)

type Observer interface {
	MessageArrived(*IttoDbMessage)
	OperationAppliedToOrders(IttoOperation)
	BeforeBookUpdate(Book, IttoOperation)
	AfterBookUpdate(Book, IttoOperation)
}

type NilObserver struct{}

func (*NilObserver) MessageArrived(*IttoDbMessage)          {}
func (*NilObserver) OperationAppliedToOrders(IttoOperation) {}
func (*NilObserver) BeforeBookUpdate(Book, IttoOperation)   {}
func (*NilObserver) AfterBookUpdate(Book, IttoOperation)    {}

type SimLogger struct {
	w              io.Writer
	tobOld, tobNew []PriceLevel
}

const SimLoggerSupernodeLevels = 32

func NewSimLogger(w io.Writer) *SimLogger {
	return &SimLogger{w: w}
}
func (s *SimLogger) printf(format string, vs ...interface{}) {
	if _, err := fmt.Fprintf(s.w, format, vs...); err != nil {
		log.Fatal("output error", err)
	}
}
func (s *SimLogger) printfln(format string, vs ...interface{}) {
	f := format + "\n"
	s.printf(f, vs...)
}
func (s *SimLogger) MessageArrived(idm *IttoDbMessage) {
	out := func(name string, typ itto.IttoMessageType, f string, vs ...interface{}) {
		s.printf("NORM %s %c ", name, typ)
		s.printfln(f, vs...)
	}
	sideChar := func(s itto.MarketSide) byte {
		if s == itto.MarketSideAsk {
			return 'S'
		}
		return byte(s)
	}
	switch im := idm.Pam.Layer().(type) {
	case *itto.IttoMessageAddOrder:
		out("ORDER", im.Type, "%c %08x %08x %08x %08x", sideChar(im.Side), im.OId, im.RefNumD.Delta(), im.Size, im.Price)
	case *itto.IttoMessageAddQuote:
		out("QBID", im.Type, "%08x %08x %08x %08x", im.OId, im.Bid.RefNumD.Delta(), im.Bid.Size, im.Bid.Price)
		out("QASK", im.Type, "%08x %08x %08x %08x", im.OId, im.Ask.RefNumD.Delta(), im.Ask.Size, im.Ask.Price)
	case *itto.IttoMessageSingleSideExecuted:
		out("ORDER", im.Type, "%08x %08x", im.OrigRefNumD.Delta(), im.Size)
	case *itto.IttoMessageSingleSideExecutedWithPrice:
		out("ORDER", im.Type, "%08x %08x", im.OrigRefNumD.Delta(), im.Size)
	case *itto.IttoMessageOrderCancel:
		out("ORDER", im.Type, "%08x %08x", im.OrigRefNumD.Delta(), im.Size)
	case *itto.IttoMessageSingleSideReplace:
		out("ORDER", im.Type, "%08x %08x %08x %08x", im.RefNumD.Delta(), im.OrigRefNumD.Delta(), im.Size, im.Price)
	case *itto.IttoMessageSingleSideDelete:
		out("ORDER", im.Type, "%08x", im.OrigRefNumD.Delta())
	case *itto.IttoMessageSingleSideUpdate:
		out("ORDER", im.Type, "%08x %08x %08x", im.RefNumD.Delta(), im.Size, im.Price)
	case *itto.IttoMessageQuoteReplace:
		out("QBID", im.Type, "%08x %08x %08x %08x", im.Bid.RefNumD.Delta(), im.Bid.OrigRefNumD.Delta(), im.Bid.Size, im.Bid.Price)
		out("QASK", im.Type, "%08x %08x %08x %08x", im.Ask.RefNumD.Delta(), im.Ask.OrigRefNumD.Delta(), im.Ask.Size, im.Ask.Price)
	case *itto.IttoMessageQuoteDelete:
		out("QBID", im.Type, "%08x", im.BidOrigRefNumD.Delta())
		out("QASK", im.Type, "%08x", im.AskOrigRefNumD.Delta())
	case *itto.IttoMessageBlockSingleSideDelete:
		for _, r := range im.RefNumDs {
			out("ORDER", im.Type, "%08x", r.Delta())
		}
	}
}
func (s *SimLogger) OperationAppliedToOrders(operation IttoOperation) {
	type ordrespLogInfo struct {
		notFound, addOp, refNum uint32
		optionId                itto.OptionId
		side, price, size       int
		ordlSuffix              string
	}
	type orduLogInfo struct {
		refNum            uint32
		optionId          itto.OptionId
		side, price, size int
	}

	var or ordrespLogInfo
	var ou orduLogInfo
	if op, ok := operation.(*OperationAdd); ok {
		or = ordrespLogInfo{
			addOp:      1,
			refNum:     op.RefNumD.Delta(),
			optionId:   op.optionId,
			ordlSuffix: fmt.Sprintf(" %08x", op.optionId),
		}
		ou = orduLogInfo{
			refNum:   or.refNum,
			optionId: op.GetOptionId(),
			price:    op.GetPrice(),
			size:     op.GetNewSize(),
		}
		if op.GetSide() == itto.MarketSideAsk {
			ou.side = 1
		}
	} else {
		if operation.GetOptionId().Invalid() {
			or = ordrespLogInfo{notFound: 1}
		} else {
			or = ordrespLogInfo{
				optionId: operation.GetOptionId(),
				price:    operation.GetPrice(),
				size:     -operation.GetSizeDelta(),
			}
			if operation.GetSide() == itto.MarketSideAsk {
				or.side = 1
			}
		}
		if operation.GetNewSize() != 0 {
			ou = orduLogInfo{
				optionId: or.optionId,
				side:     or.side,
				price:    or.price,
				size:     operation.GetNewSize(),
			}
		}
		or.refNum = operation.getOperation().origRefNumD.Delta()
		ou.refNum = or.refNum
	}
	s.printfln("ORDL %d %08x%s", or.addOp, or.refNum, or.ordlSuffix)
	s.printfln("ORDRESP %d %d %d %08x %08x %08x %08x", or.notFound, or.addOp, or.side, or.size, or.price, or.optionId, or.refNum)
	if operation.GetOptionId().Valid() {
		s.printfln("ORDU %08x %08x %d %08x %08x", ou.refNum, ou.optionId, ou.side, ou.price, ou.size)
	}
}
func (s *SimLogger) BeforeBookUpdate(book Book, operation IttoOperation) {
	s.tobOld = book.GetTop(operation.GetOptionId(), operation.GetSide(), SimLoggerSupernodeLevels)
}
func (s *SimLogger) AfterBookUpdate(book Book, operation IttoOperation) {
	if operation.GetOptionId().Invalid() {
		return
	}
	s.tobNew = book.GetTop(operation.GetOptionId(), operation.GetSide(), SimLoggerSupernodeLevels)

	empty := PriceLevel{}
	if operation.GetSide() == itto.MarketSideAsk {
		empty.Price = -1
	}
	for i := 0; i < SimLoggerSupernodeLevels; i++ {
		plo, pln := empty, empty
		if i < len(s.tobOld) {
			plo = s.tobOld[i]
		}
		if i < len(s.tobNew) {
			pln = s.tobNew[i]
		}
		s.printfln("SN_OLD_NEW %02d %08x %08x  %08x %08x", i,
			plo.Size, uint32(plo.Price),
			pln.Size, uint32(pln.Price),
		)
	}
}
