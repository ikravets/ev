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
}

type NilObserver struct{}

func (*NilObserver) MessageArrived(*IttoDbMessage)          {}
func (*NilObserver) OperationAppliedToOrders(IttoOperation) {}

type SimLogger struct {
	w io.Writer
}

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
	marketSide2int := func(ms itto.MarketSide) int {
		if ms == itto.MarketSideAsk {
			return 1
		} else {
			return 0
		}
	}
	switch op := operation.(type) {
	case *OperationAdd:
		refNum := op.RefNumD.Delta()
		s.printfln("ORDL 1 %08x %08x", refNum, op.optionId)
		s.printfln("ORDRESP 0 1 0 %08x %08x %08x %08x", 0, 0, op.optionId, refNum)
		if op.GetOptionId().Valid() {
			s.printfln("ORDU %08x %08x %d %08x %08x", refNum, op.GetOptionId(), marketSide2int(op.GetSide()), op.GetPrice(), op.GetSizeDelta())
		}
	default:
		refNum := op.getOperation().origRefNumD.Delta()
		s.printfln("ORDL 0 %08x", refNum)
		if op.GetOptionId().Valid() {
			s.printfln("ORDRESP 0 0 %d %08x %08x %08x %08x", marketSide2int(op.GetSide()), -op.GetSizeDelta(), op.GetPrice(), op.GetOptionId(), refNum)
			size := op.getOperation().origOrder.Size + op.GetSizeDelta()
			if size == 0 {
				s.printfln("ORDU %08x %08x %d %08x %08x", refNum, 0, 0, 0, 0)
			} else {
				s.printfln("ORDU %08x %08x %d %08x %08x", refNum, op.GetOptionId(), marketSide2int(op.GetSide()), op.GetPrice(), size)
			}
		} else {
			s.printfln("ORDRESP 1 0 %d %08x %08x %08x %08x", marketSide2int(op.GetSide()), 0, 0, 0, refNum)
		}
	}
}
