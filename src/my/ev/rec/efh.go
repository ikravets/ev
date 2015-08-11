// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"my/errs"

	"my/ev/packet"
	"my/ev/packet/bats"
	"my/ev/packet/nasdaq"
	"my/ev/sim"
)

const (
	EFHM_DEFINITION      = 0
	EFHM_TRADE           = 1
	EFHM_QUOTE           = 2
	EFHM_ORDER           = 3
	EFHM_DEFINITION_NOM  = 4
	EFHM_DEFINITION_BATS = 5
	EFHM_REFRESHED       = 100
	EFHM_STOPPED         = 101

	EFH_ORDER_BID = 1
	EFH_ORDER_ASK = -1

	EFH_SECURITY_PUT  = 0
	EFH_SECURITY_CALL = 1
)

type efhm_header struct {
	Type           uint8
	TickCondition  uint8
	QueuePosition  uint16
	UnderlyingId   uint32
	SecurityId     uint64
	SequenceNumber uint64
	TimeStamp      uint64
}

type efhm_order struct {
	efhm_header
	TradeStatus     uint8
	OrderType       uint8
	OrderSide       int8
	_pad            byte
	Price           uint32
	Size            uint32
	AoNSize         uint32
	CustomerSize    uint32
	CustomerAoNSize uint32
	BDSize          uint32
	BDAoNSize       uint32
}

type efhm_quote struct {
	efhm_header
	TradeStatus        uint8
	_pad               [3]byte
	BidPrice           uint32
	BidSize            uint32
	BidOrderSize       uint32
	BidAoNSize         uint32
	BidCustomerSize    uint32
	BidCustomerAoNSize uint32
	BidBDSize          uint32
	BidBDAoNSize       uint32
	AskPrice           uint32
	AskSize            uint32
	AskOrderSize       uint32
	AskAoNSize         uint32
	AskCustomerSize    uint32
	AskCustomerAoNSize uint32
	AskBDSize          uint32
	AskBDAoNSize       uint32
}

type efhm_trade struct {
	efhm_header
	Price          uint32
	Size           uint32
	TradeCondition uint8
}

type efhm_definition_nom struct {
	efhm_header
	Symbol           [8]byte
	MaturityDate     uint64
	UnderlyingSymbol [16]byte
	StrikePrice      uint32
	PutOrCall        uint8
}

type efhm_definition_bats struct {
	efhm_header
	OsiSymbol [22]byte
}

func (m efhm_header) String() string {
	switch m.Type {
	case EFHM_QUOTE, EFHM_ORDER, EFHM_TRADE, EFHM_DEFINITION_NOM, EFHM_DEFINITION_BATS:
		return fmt.Sprintf("HDR{T:%d, TC:%d, QP:%d, UId:%08x, SId:%016x, SN:%d, TS:%016x}",
			m.Type,
			m.TickCondition,
			m.QueuePosition,
			m.UnderlyingId,
			m.SecurityId,
			m.SequenceNumber,
			m.TimeStamp,
		)
	default:
		return fmt.Sprintf("HDR{T:%d}", m.Type)
	}
}
func (m efhm_order) String() string {
	return fmt.Sprintf("%s ORD{TS:%d, OT:%d, OS:%+d, P:%10d, S:%d, AS:%d, CS:%d, CAS:%d, BS:%d, BAS:%d}",
		m.efhm_header,
		m.TradeStatus,
		m.OrderType,
		m.OrderSide,
		m.Price,
		m.Size,
		m.AoNSize,
		m.CustomerSize,
		m.CustomerAoNSize,
		m.BDSize,
		m.BDAoNSize,
	)
}
func (m efhm_quote) String() string {
	return fmt.Sprintf("%s QUO{TS:%d, "+
		"Bid{P:%10d, S:%d, OS:%d, AS:%d, CS:%d, CAS:%d, BS:%d, BAS:%d}, "+
		"Ask{P:%10d, S:%d, OS:%d, AS:%d, CS:%d, CAS:%d, BS:%d, BAS:%d}"+
		"}",
		m.efhm_header,
		m.TradeStatus,
		m.BidPrice,
		m.BidSize,
		m.BidOrderSize,
		m.BidAoNSize,
		m.BidCustomerSize,
		m.BidCustomerAoNSize,
		m.BidBDSize,
		m.BidBDAoNSize,
		m.AskPrice,
		m.AskSize,
		m.AskOrderSize,
		m.AskAoNSize,
		m.AskCustomerSize,
		m.AskCustomerAoNSize,
		m.AskBDSize,
		m.AskBDAoNSize,
	)
}
func (m efhm_trade) String() string {
	return fmt.Sprintf("%s TRD{P:%10d, S:%d, TC:%d}",
		m.efhm_header,
		m.Price,
		m.Size,
		m.TradeCondition,
	)
}
func (m efhm_definition_nom) String() string {
	return fmt.Sprintf("%s DEF_NOM{S:\"%s\" %016x, MD:%x, US:\"%s\" %016x, SP:%d, PC:%d}",
		m.efhm_header,
		trimAsciiz(m.Symbol[:]),
		binary.LittleEndian.Uint64(m.Symbol[:]),
		m.MaturityDate,
		trimAsciiz(m.UnderlyingSymbol[:]),
		binary.LittleEndian.Uint64(m.UnderlyingSymbol[:]),
		m.StrikePrice,
		m.PutOrCall,
	)
}
func (m efhm_definition_bats) String() string {
	return fmt.Sprintf("%s DEF_BATS{OS:\"%s\"}",
		m.efhm_header,
		trimAsciiz(m.OsiSymbol[:]),
	)
}
func trimAsciiz(b []byte) []byte {
	pos := bytes.IndexByte(b, 0)
	if pos < 0 {
		return b
	}
	return b[:pos]
}

type EfhLoggerPrinter interface {
	PrintOrder(efhm_order) error
	PrintQuote(efhm_quote) error
	PrintTrade(efhm_trade) error
	PrintDefinitionNom(efhm_definition_nom) error
	PrintDefinitionBats(efhm_definition_bats) error
}

type testefhPrinter struct {
	w io.Writer
}

var _ EfhLoggerPrinter = &testefhPrinter{}

func NewTestefhPrinter(w io.Writer) EfhLoggerPrinter {
	return &testefhPrinter{w: w}
}
func (p *testefhPrinter) PrintOrder(o efhm_order) error {
	return p.print(o)
}
func (p *testefhPrinter) PrintQuote(o efhm_quote) error {
	return p.print(o)
}
func (p *testefhPrinter) PrintTrade(m efhm_trade) error {
	return p.print(m)
}
func (p *testefhPrinter) PrintDefinitionNom(m efhm_definition_nom) error {
	return p.print(m)
}
func (p *testefhPrinter) PrintDefinitionBats(m efhm_definition_bats) error {
	return p.print(m)
}
func (p *testefhPrinter) print(v interface{}) error {
	_, err := fmt.Fprintln(p.w, v)
	return err
}

type EfhLogger struct {
	TobLogger
	printer EfhLoggerPrinter
	mode    EfhLoggerOutputMode
	stream  Stream
}

var _ sim.Observer = &EfhLogger{}

func NewEfhLogger(p EfhLoggerPrinter) *EfhLogger {
	l := &EfhLogger{
		printer:   p,
		TobLogger: *NewTobLogger(),
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
	l.TobLogger.MessageArrived(idm)
	switch m := l.stream.getExchangeMessage().(type) {
	case packet.TradeMessage:
		l.genUpdateTrades(m)
	case *nasdaq.IttoMessageOptionDirectory:
		l.genUpdateDefinitionsNom(m)
	case *bats.PitchMessageSymbolMapping:
		l.genUpdateDefinitionsBats(m)
	}
}

func (l *EfhLogger) AfterBookUpdate(book sim.Book, operation sim.SimOperation) {
	if l.mode == EfhLoggerOutputOrders {
		if l.TobLogger.AfterBookUpdate(book, operation, TobUpdateNew) {
			l.genUpdateOrders(l.TobLogger.bid)
			l.genUpdateOrders(l.TobLogger.ask)
		}
	} else {
		if l.TobLogger.AfterBookUpdate(book, operation, TobUpdateNewForce) {
			l.genUpdateQuotes(l.TobLogger.bid, l.TobLogger.ask)
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
	return l.genUpdateHeaderForOption(messageType, l.TobLogger.lastOptionId)
}
func (l *EfhLogger) genUpdateOrders(tob tob) {
	if !tob.updated() {
		return
	}
	m := efhm_order{
		efhm_header: l.genUpdateHeader(EFHM_ORDER),
		Price:       uint32(tob.New.Price),
		Size:        uint32(tob.New.Size),
		OrderType:   1,
	}
	switch tob.Side {
	case packet.MarketSideBid:
		m.OrderSide = EFH_ORDER_BID
	case packet.MarketSideAsk:
		m.OrderSide = EFH_ORDER_ASK
	}
	errs.CheckE(l.printer.PrintOrder(m))
}
func (l *EfhLogger) genUpdateQuotes(bid, ask tob) {
	m := efhm_quote{
		efhm_header: l.genUpdateHeader(EFHM_QUOTE),
		BidPrice:    uint32(bid.New.Price),
		BidSize:     uint32(bid.New.Size),
		AskPrice:    uint32(ask.New.Price),
		AskSize:     uint32(ask.New.Size),
	}
	errs.CheckE(l.printer.PrintQuote(m))
}
func (l *EfhLogger) genUpdateTrades(msg packet.TradeMessage) {
	oid, price, size := msg.TradeInfo()
	m := efhm_trade{
		efhm_header: l.genUpdateHeaderForOption(EFHM_TRADE, oid),
		Price:       uint32(packet.PriceTo4Dec(price)),
		Size:        uint32(size),
	}
	errs.CheckE(l.printer.PrintTrade(m))
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
	errs.CheckE(l.printer.PrintDefinitionNom(m))
}
func (l *EfhLogger) genUpdateDefinitionsBats(msg *bats.PitchMessageSymbolMapping) {
	m := efhm_definition_bats{
		efhm_header: l.genUpdateHeaderForOption(EFHM_DEFINITION_BATS, msg.OptionId()),
	}
	m.efhm_header.SequenceNumber = 0
	m.efhm_header.TimeStamp = 0
	copy(m.OsiSymbol[:], msg.OsiSymbol)
	errs.CheckE(l.printer.PrintDefinitionBats(m))
}
