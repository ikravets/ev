// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"fmt"
	"io"

	"my/itto/verify/packet/itto"
	"my/itto/verify/sim"
)

const (
	EFHM_DEFINITION = 0
	EFHM_TRADE      = 1
	EFHM_QUOTE      = 2
	EFHM_ORDER      = 3
	EFHM_REFRESHED  = 100
	EFHM_STOPPED    = 101
)
const (
	EFH_ORDER_BID = 1
	EFH_ORDER_ASK = -1
)

type efhm_header struct {
	Type           uint8
	TickCondition  uint8
	QueuePosition  uint16
	UnderlyingId   uint32
	SecurityId     uint32
	SequenceNumber uint32
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

func (m efhm_header) String() string {
	switch m.Type {
	case EFHM_QUOTE, EFHM_ORDER, EFHM_TRADE:
		return fmt.Sprintf("HDR{T:%d, TC:%d, QP:%d, UId:%08x, SId:%08x, SN:%d, TS:%08x}",
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

type EfhLoggerPrinter interface {
	PrintOrder(efhm_order)
	PrintQuote(efhm_quote)
	PrintTrade(efhm_trade)
}

type testefhPrinter struct {
	w io.Writer
}

var _ EfhLoggerPrinter = &testefhPrinter{}

func NewTestefhPrinter(w io.Writer) EfhLoggerPrinter {
	return &testefhPrinter{w: w}
}
func (p *testefhPrinter) PrintOrder(o efhm_order) {
	fmt.Fprintln(p.w, o)
}
func (p *testefhPrinter) PrintQuote(o efhm_quote) {
	fmt.Fprintln(p.w, o)
}
func (p *testefhPrinter) PrintTrade(m efhm_trade) {
	fmt.Fprintln(p.w, m)
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
	switch m := l.stream.getIttoMessage().(type) {
	case *itto.IttoMessageOptionsTrade:
		l.lastOptionId = m.OId
		l.genUpdateTrades(m.Price, m.Size)
	case *itto.IttoMessageOptionsCrossTrade:
		l.lastOptionId = m.OId
		l.genUpdateTrades(m.Price, m.Size)
	}
}

func (l *EfhLogger) AfterBookUpdate(book sim.Book, operation sim.SimOperation) {
	if l.mode == EfhLoggerOutputOrders {
		if l.TobLogger.AfterBookUpdate(book, operation, TobUpdateNew) {
			l.genUpdateOrders(l.bid)
			l.genUpdateOrders(l.ask)
		}
	} else {
		if l.TobLogger.AfterBookUpdate(book, operation, TobUpdateNewForce) {
			l.genUpdateQuotes()
		}
	}
}

func (l *EfhLogger) genUpdateHeader(messageType uint8) efhm_header {
	return efhm_header{
		Type:           messageType,
		SecurityId:     uint32(l.lastOptionId),
		SequenceNumber: uint32(l.stream.getSeqNum()), // FIXME MoldUDP64 seqNum is 64 bit
		TimeStamp:      l.stream.getTimestamp(),
	}
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
	case itto.MarketSideBid:
		m.OrderSide = EFH_ORDER_BID
	case itto.MarketSideAsk:
		m.OrderSide = EFH_ORDER_ASK
	}
	l.printer.PrintOrder(m)
}
func (l *EfhLogger) genUpdateQuotes() {
	m := efhm_quote{
		efhm_header: l.genUpdateHeader(EFHM_QUOTE),
		BidPrice:    uint32(l.bid.New.Price),
		BidSize:     uint32(l.bid.New.Size),
		AskPrice:    uint32(l.ask.New.Price),
		AskSize:     uint32(l.ask.New.Size),
	}
	l.printer.PrintQuote(m)
}
func (l *EfhLogger) genUpdateTrades(price, size int) {
	m := efhm_trade{
		efhm_header: l.genUpdateHeader(EFHM_TRADE),
		Price:       uint32(price),
		Size:        uint32(size),
	}
	l.printer.PrintTrade(m)
}
