// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	EFHM_DEFINITION      = 0
	EFHM_TRADE           = 1
	EFHM_QUOTE           = 2
	EFHM_ORDER           = 3
	EFHM_DEFINITION_NOM  = 4
	EFHM_DEFINITION_BATS = 5
	EFHM_DEFINITION_MIAX = 6
	EFHM_REFRESHED       = 100
	EFHM_STOPPED         = 101

	EFH_ORDER_BID = 1
	EFH_ORDER_ASK = -1

	EFH_SECURITY_PUT  = 0
	EFH_SECURITY_CALL = 1
)

var efhmOutputNames = [...]string{
	EFHM_DEFINITION:      "",
	EFHM_TRADE:           "TRD",
	EFHM_QUOTE:           "QUO",
	EFHM_ORDER:           "ORD",
	EFHM_DEFINITION_NOM:  "DEF_NOM",
	EFHM_DEFINITION_BATS: "DEF_BATS",
	EFHM_DEFINITION_MIAX: "DEF_MIAX",
}

type efhm_header struct {
	Type           uint8
	GroupId        uint8
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
	case EFHM_QUOTE, EFHM_ORDER, EFHM_TRADE, EFHM_DEFINITION_NOM, EFHM_DEFINITION_BATS, EFHM_DEFINITION_MIAX:
		return fmt.Sprintf("HDR{T:%d, G:%d, QP:%d, UId:%08x, SId:%016x, SN:%d, TS:%016x} %s",
			m.Type,
			m.GroupId,
			m.QueuePosition,
			m.UnderlyingId,
			m.SecurityId,
			m.SequenceNumber,
			m.TimeStamp,
			efhmOutputNames[m.Type],
		)
	default:
		return fmt.Sprintf("HDR{T:%d}", m.Type)
	}
}
func (m efhm_order) String() string {
	return fmt.Sprintf("%s{TS:%d, OT:%d, OS:%+d, P:%10d, S:%d, AS:%d, CS:%d, CAS:%d, BS:%d, BAS:%d}",
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
	return fmt.Sprintf("%s{TS:%d, "+
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
	return fmt.Sprintf("%s{P:%10d, S:%d, TC:%d}",
		m.efhm_header,
		m.Price,
		m.Size,
		m.TradeCondition,
	)
}
func (m efhm_definition_nom) String() string {
	return fmt.Sprintf("%s{S:\"%s\" %016x, MD:%x, US:\"%s\" %016x, SP:%d, PC:%d}",
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
	return fmt.Sprintf("%s{OS:\"%s\"}",
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

type efhMessage interface {
	fmt.Stringer
	efhMessage()
}

func (h efhm_header) efhMessage() {}
