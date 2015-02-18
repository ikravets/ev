// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package efhsim

import (
	"fmt"
	"io"
	"log"

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

func (m efhm_header) String() string {
	switch m.Type {
	case EFHM_QUOTE, EFHM_ORDER:
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

var _ sim.Observer = &EfhLogger{}

type EfhLogger struct {
	w            io.Writer
	lastMessage  *sim.IttoDbMessage
	lastOptionId itto.OptionId
	consumeOps   int
	curOps       int
	ittoSeconds  uint32
	mode         EfhLoggerOutputMode
	bid          tob
	ask          tob
}
type tob struct {
	Check bool
	Side  itto.MarketSide
	Old   sim.PriceLevel
	New   sim.PriceLevel
}

func NewEfhLogger(w io.Writer) *EfhLogger {
	l := &EfhLogger{w: w,
		bid: tob{Side: itto.MarketSideBid},
		ask: tob{Side: itto.MarketSideAsk},
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

func (l *EfhLogger) MessageArrived(idm *sim.IttoDbMessage) {
	l.lastMessage = idm
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
	default:
		log.Println("wrong message type ", l.lastMessage.Pam.Layer())
		return
	}
	l.curOps = 0
}

func (*EfhLogger) OperationAppliedToOrders(sim.IttoOperation) {}

func (l *EfhLogger) BeforeBookUpdate(book sim.Book, operation sim.IttoOperation) {
	if l.curOps > 0 {
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
}

func (l *EfhLogger) AfterBookUpdate(book sim.Book, operation sim.IttoOperation) {
	l.curOps++
	if l.curOps < l.consumeOps {
		return
	}
	l.curOps = 0
	if l.lastOptionId.Invalid() {
		return
	}

	u := TobUpdateNew
	if l.mode == EfhLoggerOutputQuotes {
		u = TobUpdateNewForce
	}
	l.bid.update(book, l.lastOptionId, u)
	l.ask.update(book, l.lastOptionId, u)
	if l.mode == EfhLoggerOutputOrders {
		l.genUpdateOrders(l.bid)
		l.genUpdateOrders(l.ask)
	} else {
		l.genUpdateQuotes()
	}
}

func (l *EfhLogger) genUpdateOrders(tob tob) {
	if !tob.updated() {
		return
	}
	eo := efhm_order{
		efhm_header: l.genUpdateHeader(EFHM_ORDER),
		Price:       uint32(tob.New.Price),
		Size:        uint32(tob.New.Size),
		OrderType:   1,
	}
	switch tob.Side {
	case itto.MarketSideBid:
		eo.OrderSide = EFH_ORDER_BID
	case itto.MarketSideAsk:
		eo.OrderSide = EFH_ORDER_ASK
	}
	fmt.Fprintln(l.w, eo)
}

func (l *EfhLogger) genUpdateQuotes() {
	if !l.bid.updated() && !l.ask.updated() {
		return
	}
	eq := efhm_quote{
		efhm_header: l.genUpdateHeader(EFHM_QUOTE),
		BidPrice:    uint32(l.bid.New.Price),
		BidSize:     uint32(l.bid.New.Size),
		AskPrice:    uint32(l.ask.New.Price),
		AskSize:     uint32(l.ask.New.Size),
	}
	fmt.Fprintln(l.w, eq)
}

func (l *EfhLogger) genUpdateHeader(messageType uint8) efhm_header {
	return efhm_header{
		Type:           messageType,
		SecurityId:     uint32(l.lastOptionId),
		SequenceNumber: uint32(l.lastMessage.Pam.SequenceNumber()), // FIXME MoldUDP64 seqNum is 64 bit
		TimeStamp:      uint64(l.ittoSeconds)*1e9 + uint64(l.lastMessage.Pam.Layer().(itto.IttoMessageCommon).Base().Timestamp),
	}
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
