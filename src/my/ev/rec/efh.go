// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"fmt"
	"io"
	"time"

	"github.com/ikravets/errs"

	"my/ev/packet"
	"my/ev/packet/bats"
	"my/ev/packet/miax"
	"my/ev/packet/nasdaq"
	"my/ev/sim"
)

type EfhLoggerPrinter interface {
	PrintMessage(efhMessage) error
}

type testefhPrinter struct {
	w io.Writer
}

var _ EfhLoggerPrinter = &testefhPrinter{}

func NewTestefhPrinter(w io.Writer) EfhLoggerPrinter {
	return &testefhPrinter{w: w}
}
func (p *testefhPrinter) PrintMessage(m efhMessage) error {
	_, err := fmt.Fprintln(p.w, m)
	return err
}

type EfhLogger struct {
	tobLogger TobLogger
	printer   EfhLoggerPrinter
	mode      EfhLoggerOutputMode
	stream    Stream
}

var _ sim.Observer = &EfhLogger{}

func NewEfhLogger(p EfhLoggerPrinter) *EfhLogger {
	l := &EfhLogger{
		printer:   p,
		tobLogger: *NewTobLogger(),
		stream:    *NewStream(),
	}
	return l
}

type EfhLoggerOutputMode byte

const (
	EfhLoggerOutputOrders EfhLoggerOutputMode = iota
	EfhLoggerOutputQuotes
)

func (l *EfhLogger) SetOutputMode(mode EfhLoggerOutputMode) {
	l.mode = mode
}

func (l *EfhLogger) MessageArrived(idm *sim.SimMessage) {
	l.stream.MessageArrived(idm)
	l.tobLogger.MessageArrived(idm)
	switch m := l.stream.getExchangeMessage().(type) {
	case packet.TradeMessage:
		l.genUpdateTrades(m)
	case *nasdaq.IttoMessageOptionDirectory:
		l.genUpdateDefinitionsNom(m)
	case *bats.PitchMessageSymbolMapping:
		l.genUpdateDefinitionsBats(m)
	case *miax.TomMessageSeriesUpdate:
		l.genUpdateDefinitionsMiax(m)
	}
}
func (l *EfhLogger) OperationAppliedToOrders(operation sim.SimOperation) {
	l.tobLogger.OperationAppliedToOrders(operation)
}
func (l *EfhLogger) BeforeBookUpdate(book sim.Book, operation sim.SimOperation) {
	l.tobLogger.BeforeBookUpdate(book, operation)
}
func (l *EfhLogger) AfterBookUpdate(book sim.Book, operation sim.SimOperation) {
	if l.mode == EfhLoggerOutputOrders {
		if l.tobLogger.AfterBookUpdate(book, operation, TobUpdateNew) {
			l.genUpdateOrders(l.tobLogger.bid)
			l.genUpdateOrders(l.tobLogger.ask)
		}
	} else {
		if l.tobLogger.AfterBookUpdate(book, operation, TobUpdateNewForce) {
			l.genUpdateQuotes(l.tobLogger.bid, l.tobLogger.ask)
		}
	}
}

func (l *EfhLogger) genUpdateHeaderForOption(messageType uint8, oid packet.OptionId) efhm_header {
	return efhm_header{
		Type:           messageType,
		SecurityId:     oid.ToUint64(),
		SequenceNumber: l.stream.getSeqNum(),
		TimeStamp:      l.stream.getTimestamp(),
	}
}
func (l *EfhLogger) genUpdateHeader(messageType uint8) efhm_header {
	return l.genUpdateHeaderForOption(messageType, l.tobLogger.lastOptionId)
}
func (l *EfhLogger) genUpdateOrders(tob tob) {
	if !tob.updated() {
		return
	}
	m := efhm_order{
		efhm_header:     l.genUpdateHeader(EFHM_ORDER),
		Price:           uint32(tob.New.Price()),
		Size:            uint32(tob.New.Size(sim.SizeKindDefault)),
		AoNSize:         uint32(tob.New.Size(sim.SizeKindAON)),
		CustomerSize:    uint32(tob.New.Size(sim.SizeKindCustomer)),
		CustomerAoNSize: uint32(tob.New.Size(sim.SizeKindCustomerAON)),
		OrderType:       1,
	}
	switch tob.Side {
	case packet.MarketSideBid:
		m.OrderSide = EFH_ORDER_BID
	case packet.MarketSideAsk:
		m.OrderSide = EFH_ORDER_ASK
	}
	errs.CheckE(l.printer.PrintMessage(m))
}
func (l *EfhLogger) genUpdateQuotes(bid, ask tob) {
	m := efhm_quote{
		efhm_header:        l.genUpdateHeader(EFHM_QUOTE),
		BidPrice:           uint32(bid.New.Price()),
		BidSize:            uint32(bid.New.Size(sim.SizeKindDefault)),
		BidAoNSize:         uint32(bid.New.Size(sim.SizeKindAON)),
		BidCustomerSize:    uint32(bid.New.Size(sim.SizeKindCustomer)),
		BidCustomerAoNSize: uint32(bid.New.Size(sim.SizeKindCustomerAON)),
		AskPrice:           uint32(ask.New.Price()),
		AskSize:            uint32(ask.New.Size(sim.SizeKindDefault)),
		AskAoNSize:         uint32(ask.New.Size(sim.SizeKindAON)),
		AskCustomerSize:    uint32(ask.New.Size(sim.SizeKindCustomer)),
		AskCustomerAoNSize: uint32(ask.New.Size(sim.SizeKindCustomerAON)),
	}
	errs.CheckE(l.printer.PrintMessage(m))
}
func (l *EfhLogger) genUpdateTrades(msg packet.TradeMessage) {
	oid, price, size := msg.TradeInfo()
	m := efhm_trade{
		efhm_header: l.genUpdateHeaderForOption(EFHM_TRADE, oid),
		Price:       uint32(packet.PriceTo4Dec(price)),
		Size:        uint32(size),
	}
	errs.CheckE(l.printer.PrintMessage(m))
}
func (l *EfhLogger) genUpdateDefinitionsNom(msg *nasdaq.IttoMessageOptionDirectory) {
	m := efhm_definition_nom{
		efhm_header: l.genUpdateHeaderForOption(EFHM_DEFINITION_NOM, msg.OptionId()),
		StrikePrice: uint32(msg.StrikePrice),
	}
	year, month, day := msg.Expiration.Date()
	m.MaturityDate = uint64(day<<16 + int(month)<<8 + year%100)
	copy(m.Symbol[:], msg.Symbol)
	copy(m.UnderlyingSymbol[:], msg.UnderlyingSymbol)
	switch msg.OType {
	case 'C':
		m.PutOrCall = EFH_SECURITY_CALL
	case 'P':
		m.PutOrCall = EFH_SECURITY_PUT
	}
	errs.CheckE(l.printer.PrintMessage(m))
}

// FIXME boilerplate code left until there's guarantee that output for MIAX should have exactly the same fields as for NASDAQ
func (l *EfhLogger) genUpdateDefinitionsMiax(msg *miax.TomMessageSeriesUpdate) {
	m := efhm_definition_nom{
		efhm_header: l.genUpdateHeaderForOption(EFHM_DEFINITION_MIAX, msg.OptionId()),
		StrikePrice: uint32(msg.StrikePrice),
	}
	t, ok := time.Parse("20060102", msg.Expiration)
	errs.CheckE(ok)
	year, month, day := t.Date()
	m.MaturityDate = uint64(day<<16 + int(month)<<8 + year%100)
	copy(m.Symbol[:], msg.SecuritySymbol)
	copy(m.UnderlyingSymbol[:], msg.UnderlyingSymbol)
	switch msg.CallOrPut {
	case 'C':
		m.PutOrCall = EFH_SECURITY_CALL
	case 'P':
		m.PutOrCall = EFH_SECURITY_PUT
	}
	errs.CheckE(l.printer.PrintMessage(m))
}
func (l *EfhLogger) genUpdateDefinitionsBats(msg *bats.PitchMessageSymbolMapping) {
	m := efhm_definition_bats{
		efhm_header: l.genUpdateHeaderForOption(EFHM_DEFINITION_BATS, msg.OptionId()),
	}
	m.efhm_header.SequenceNumber = 0
	m.efhm_header.TimeStamp = 0
	copy(m.OsiSymbol[:], msg.OsiSymbol)
	errs.CheckE(l.printer.PrintMessage(m))
}
