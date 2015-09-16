// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ikravets/errs"

	"my/ev/packet"
	"my/ev/packet/bats"
	"my/ev/packet/miax"
	"my/ev/packet/nasdaq"
	"my/ev/sim"
)

type SimLogger struct {
	w               io.Writer
	tobOld, tobNew  []sim.PriceLevel
	efhLogger       EfhLogger
	supernodeLevels int
}

const SimLoggerDefaultSupernodeLevels = 256

func NewSimLogger(w io.Writer) *SimLogger {
	s := &SimLogger{
		w:               w,
		supernodeLevels: SimLoggerDefaultSupernodeLevels,
	}
	s.efhLogger = *NewEfhLogger(s)
	return s
}
func (s *SimLogger) SetOutputMode(mode EfhLoggerOutputMode) {
	s.efhLogger.SetOutputMode(mode)
}
func (s *SimLogger) SetSupernodeLevels(levels int) {
	errs.Check(levels > 0)
	s.supernodeLevels = levels
}

func (s *SimLogger) printf(format string, vs ...interface{}) {
	_, err := fmt.Fprintf(s.w, format, vs...)
	errs.CheckE(err)
}
func (s *SimLogger) printfln(format string, vs ...interface{}) {
	f := format + "\n"
	s.printf(f, vs...)
}
func (s *SimLogger) MessageArrived(idm *sim.SimMessage) {
	outItto := func(name string, typ nasdaq.IttoMessageType, f string, vs ...interface{}) {
		s.printf("NORM %s %c ", name, typ)
		s.printfln(f, vs...)
	}
	outBats := func(f string, vs ...interface{}) {
		s.printf("NORM ORDER %02x ", idm.Pam.Layer().(bats.PitchMessage).Base().Type.ToInt())
		s.printfln(f, vs...)
	}
	outMiax := func(f string, vs ...interface{}) {
		s.printf("NORM TOM %02x ", idm.Pam.Layer().(miax.TomMessage).Base().Type.ToInt())
		s.printfln(f, vs...)
	}
	sideChar := func(s packet.MarketSide) byte {
		if s == packet.MarketSideAsk {
			return 'S'
		}
		return byte(s)
	}
	switch im := idm.Pam.Layer().(type) {
	case *nasdaq.IttoMessageAddOrder:
		outItto("ORDER", im.Type, "%c %08x %08x %08x %08x", sideChar(im.Side), im.OId, im.RefNumD.ToUint32(), im.Size, im.Price)
	case *nasdaq.IttoMessageAddQuote:
		outItto("QBID", im.Type, "%08x %08x %08x %08x", im.OId, im.Bid.RefNumD.ToUint32(), im.Bid.Size, im.Bid.Price)
		outItto("QASK", im.Type, "%08x %08x %08x %08x", im.OId, im.Ask.RefNumD.ToUint32(), im.Ask.Size, im.Ask.Price)
	case *nasdaq.IttoMessageSingleSideExecuted:
		outItto("ORDER", im.Type, "%08x %08x", im.OrigRefNumD.ToUint32(), im.Size)
	case *nasdaq.IttoMessageSingleSideExecutedWithPrice:
		outItto("ORDER", im.Type, "%08x %08x", im.OrigRefNumD.ToUint32(), im.Size)
	case *nasdaq.IttoMessageOrderCancel:
		outItto("ORDER", im.Type, "%08x %08x", im.OrigRefNumD.ToUint32(), im.Size)
	case *nasdaq.IttoMessageSingleSideReplace:
		outItto("ORDER", im.Type, "%08x %08x %08x %08x", im.RefNumD.ToUint32(), im.OrigRefNumD.ToUint32(), im.Size, im.Price)
	case *nasdaq.IttoMessageSingleSideDelete:
		outItto("ORDER", im.Type, "%08x", im.OrigRefNumD.ToUint32())
	case *nasdaq.IttoMessageSingleSideUpdate:
		outItto("ORDER", im.Type, "%08x %08x %08x", im.RefNumD.ToUint32(), im.Size, im.Price)
	case *nasdaq.IttoMessageQuoteReplace:
		outItto("QBID", im.Type, "%08x %08x %08x %08x", im.Bid.RefNumD.ToUint32(), im.Bid.OrigRefNumD.ToUint32(), im.Bid.Size, im.Bid.Price)
		outItto("QASK", im.Type, "%08x %08x %08x %08x", im.Ask.RefNumD.ToUint32(), im.Ask.OrigRefNumD.ToUint32(), im.Ask.Size, im.Ask.Price)
	case *nasdaq.IttoMessageQuoteDelete:
		outItto("QBID", im.Type, "%08x", im.BidOrigRefNumD.ToUint32())
		outItto("QASK", im.Type, "%08x", im.AskOrigRefNumD.ToUint32())
	case *nasdaq.IttoMessageBlockSingleSideDelete:
		for _, r := range im.RefNumDs {
			outItto("ORDER", im.Type, "%08x", r.ToUint32())
		}

	case *bats.PitchMessageAddOrder:
		outBats("%c %012x %016x %08x %08x", sideChar(im.Side), im.Symbol.ToUint64(), im.OrderId.ToUint64(), im.Size, packet.PriceTo4Dec(im.Price))
	case *bats.PitchMessageDeleteOrder:
		outBats("%016x", im.OrderId.ToUint64())
	case *bats.PitchMessageOrderExecuted:
		outBats("%016x %08x", im.OrderId.ToUint64(), im.Size)
	case *bats.PitchMessageOrderExecutedAtPriceSize:
		outBats("%016x %08x", im.OrderId.ToUint64(), im.Size)
	case *bats.PitchMessageReduceSize:
		outBats("%016x %08x", im.OrderId.ToUint64(), im.Size)
	case *bats.PitchMessageModifyOrder:
		outBats("%016x %08x %08x", im.OrderId.ToUint64(), im.Size, packet.PriceTo4Dec(im.Price))
	case *miax.TomMessageTom:
		outMiax("%c %08x %08x %08x %08x", sideChar(im.Side), im.ProductId.ToUint32(), packet.PriceTo4Dec(im.Price), im.Size, im.PriorityCustomerSize)
	}
	s.efhLogger.MessageArrived(idm)
}
func (s *SimLogger) OperationAppliedToOrders(operation sim.SimOperation) {
	type ordrespLogInfo struct {
		notFound, addOp   int
		orderId           packet.OrderId
		optionId          packet.OptionId
		side, price, size int
		ordlSuffix        string
	}
	type orduLogInfo struct {
		orderId           packet.OrderId
		optionId          packet.OptionId
		side, price, size int
	}

	var or ordrespLogInfo
	var ou orduLogInfo
	if _, ok := operation.(*sim.OperationTop); ok {
		return
	} else if op, ok := operation.(*sim.OperationAdd); ok {
		var oid packet.OptionId
		if op.Independent() {
			oid = op.GetOptionId()
		}
		or = ordrespLogInfo{
			addOp:      1,
			orderId:    op.OrderId,
			optionId:   oid,
			ordlSuffix: fmt.Sprintf(" %012x", oid.ToUint64()),
		}
		ou = orduLogInfo{
			orderId:  or.orderId,
			optionId: op.GetOptionId(),
			price:    op.GetPrice(),
			size:     op.GetNewSize(),
		}
		if op.GetSide() == packet.MarketSideAsk {
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
			if operation.GetSide() == packet.MarketSideAsk {
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
		or.orderId = operation.GetOrigOrderId()
		ou.orderId = or.orderId
	}
	s.printfln("ORDL %d %016x%s", or.addOp, or.orderId.ToUint64(), or.ordlSuffix)
	s.printfln("ORDRESP %d %d %d %08x %08x %012x %016x", or.notFound, or.addOp, or.side, or.size, or.price, or.optionId.ToUint64(), or.orderId.ToUint64())
	if operation.GetOptionId().Valid() {
		s.printfln("ORDU %016x %012x %d %08x %08x", ou.orderId.ToUint64(), ou.optionId.ToUint64(), ou.side, ou.price, ou.size)
	}
}
func (s *SimLogger) BeforeBookUpdate(book sim.Book, operation sim.SimOperation) {
	s.tobOld = book.GetTop(operation.GetOptionId(), operation.GetSide(), s.supernodeLevels)
	s.efhLogger.BeforeBookUpdate(book, operation)
}
func (s *SimLogger) AfterBookUpdate(book sim.Book, operation sim.SimOperation) {
	if operation.GetOptionId().Valid() {
		s.tobNew = book.GetTop(operation.GetOptionId(), operation.GetSide(), s.supernodeLevels)
		empty := sim.PriceLevel{}
		if operation.GetSide() == packet.MarketSideAsk {
			empty.Price = -1
		}
		for i := 0; i < s.supernodeLevels; i++ {
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

func (s *SimLogger) PrintOrder(m efhm_order) error {
	return s.genAppUpdate(m)
}
func (s *SimLogger) PrintQuote(m efhm_quote) error {
	return s.genAppUpdate(m)
}
func (s *SimLogger) PrintTrade(m efhm_trade) error {
	return s.genAppUpdate(m)
}
func (s *SimLogger) PrintDefinitionNom(m efhm_definition_nom) error {
	return s.genAppUpdate(m)
}
func (s *SimLogger) PrintDefinitionBats(m efhm_definition_bats) error {
	return s.genAppUpdate(m)
}

func (s *SimLogger) genAppUpdate(appMessage interface{}) (err error) {
	defer errs.PassE(&err)
	var bb bytes.Buffer
	errs.CheckE(binary.Write(&bb, binary.LittleEndian, appMessage))
	if r := bb.Len() % 8; r > 0 {
		// pad to  multiple of 8 bytes
		z := make([]byte, 8)
		_, err = bb.Write(z[0 : 8-r])
		errs.CheckE(err)
	}

	for {
		var qw uint64
		if err := binary.Read(&bb, binary.LittleEndian, &qw); err != nil {
			if err == io.EOF {
				break
			}
			errs.CheckE(err)
		} else {
			s.printfln("DMATOHOST_DATA %016x", qw)
		}
	}
	s.printfln("DMATOHOST_TRAILER 00656e696c616b45")
	return
}
