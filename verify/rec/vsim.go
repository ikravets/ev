// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"

	"my/itto/verify/packet/itto"
	"my/itto/verify/sim"
)

type SimLogger struct {
	w              io.Writer
	tobOld, tobNew []sim.PriceLevel
	efhLogger      EfhLogger
}

const SimLoggerSupernodeLevels = 256

func NewSimLogger(w io.Writer) *SimLogger {
	s := &SimLogger{w: w}
	s.efhLogger = *NewEfhLogger(s)
	return s
}
func (s *SimLogger) SetOutputMode(mode EfhLoggerOutputMode) {
	s.efhLogger.SetOutputMode(mode)
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
func (s *SimLogger) MessageArrived(idm *sim.SimMessage) {
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
	s.efhLogger.MessageArrived(idm)
}
func (s *SimLogger) OperationAppliedToOrders(operation sim.SimOperation) {
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
	if op, ok := operation.(*sim.OperationAdd); ok {
		var oid itto.OptionId
		if op.Independent() {
			oid = op.GetOptionId()
		}
		or = ordrespLogInfo{
			addOp:      1,
			refNum:     op.RefNumD.Delta(),
			optionId:   oid,
			ordlSuffix: fmt.Sprintf(" %08x", oid),
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
				size:     operation.GetNewSize() - operation.GetSizeDelta(),
			}
			if operation.GetSide() == itto.MarketSideAsk {
				or.side = 1
			}
			if operation.GetNewSize() != 0 {
				ou = orduLogInfo{
					optionId: or.optionId,
					side:     or.side,
					price:    or.price,
					size:     operation.GetNewSize(),
				}
			}
		}
		or.refNum = operation.GetOrigRef().Delta()
		ou.refNum = or.refNum
	}
	s.printfln("ORDL %d %08x%s", or.addOp, or.refNum, or.ordlSuffix)
	s.printfln("ORDRESP %d %d %d %08x %08x %08x %08x", or.notFound, or.addOp, or.side, or.size, or.price, or.optionId, or.refNum)
	if operation.GetOptionId().Valid() {
		s.printfln("ORDU %08x %08x %d %08x %08x", ou.refNum, ou.optionId, ou.side, ou.price, ou.size)
	}
}
func (s *SimLogger) BeforeBookUpdate(book sim.Book, operation sim.SimOperation) {
	s.tobOld = book.GetTop(operation.GetOptionId(), operation.GetSide(), SimLoggerSupernodeLevels)
	s.efhLogger.BeforeBookUpdate(book, operation)
}
func (s *SimLogger) AfterBookUpdate(book sim.Book, operation sim.SimOperation) {
	if operation.GetOptionId().Valid() {
		s.tobNew = book.GetTop(operation.GetOptionId(), operation.GetSide(), SimLoggerSupernodeLevels)
		empty := sim.PriceLevel{}
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
	s.efhLogger.AfterBookUpdate(book, operation)
}

func (s *SimLogger) PrintOrder(m efhm_order) {
	s.genAppUpdate(m)
}
func (s *SimLogger) PrintQuote(m efhm_quote) {
	s.genAppUpdate(m)
}
func (s *SimLogger) PrintTrade(m efhm_trade) {
	s.genAppUpdate(m)
}

func (s *SimLogger) genAppUpdate(appMessage interface{}) {
	var bb bytes.Buffer
	if err := binary.Write(&bb, binary.LittleEndian, appMessage); err != nil {
		log.Fatal(err)
	}
	if r := bb.Len() % 8; r > 0 {
		// pad to  multiple of 8 bytes
		z := make([]byte, 8)
		bb.Write(z[0 : 8-r])
	}

	for {
		var qw uint64
		if err := binary.Read(&bb, binary.LittleEndian, &qw); err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		} else {
			s.printfln("DMATOHOST_DATA %016x", qw)
		}
	}
	s.printfln("DMATOHOST_TRAILER 00656e696c616b45")
}
