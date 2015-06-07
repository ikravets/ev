// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"errors"
	"fmt"
)

type MarketSide byte

const (
	MarketSideUnknown MarketSide = 0
	MarketSideBid     MarketSide = 'B'
	MarketSideAsk     MarketSide = 'A'
)

var MarketSideUnknownError = errors.New("MarketSide unknown")

func (ms MarketSide) String() string {
	switch ms {
	case MarketSideBid:
		return "B"
	case MarketSideAsk:
		return "A"
	default:
		return "?"
	}
}
func (ms MarketSide) ToByte() (byte, error) {
	switch ms {
	case MarketSideBid:
		return 'B', nil
	case MarketSideAsk:
		return 'S', nil
	default:
		return 0, MarketSideUnknownError
	}
}
func MarketSideFromByte(b byte) MarketSide {
	switch b {
	case 'B':
		return MarketSideBid
	case 'A', 'S':
		return MarketSideAsk
	default:
		return MarketSideUnknown
	}
}

type OptionId struct {
	raw uint64
}

var OptionIdUnknown = OptionId{}

func OptionIdFromUint32(v uint32) OptionId {
	return OptionId{raw: uint64(v)}
}
func OptionIdFromUint64(v uint64) OptionId {
	return OptionId{raw: v}
}
func (oid OptionId) Valid() bool {
	return oid.raw != 0
}
func (oid OptionId) Invalid() bool {
	return !oid.Valid()
}
func (oid OptionId) String() string {
	return fmt.Sprintf("%#x", oid.raw)
}
func (oid OptionId) ToUint32() uint32 {
	return uint32(oid.raw)
}
func (oid OptionId) ToUint64() uint64 {
	return oid.raw
}

type OrderId struct {
	raw uint64
}

var OrderIdUnknown = OrderId{}

func OrderIdFromUint32(v uint32) OrderId {
	return OrderId{raw: uint64(v)}
}
func OrderIdFromUint64(v uint64) OrderId {
	return OrderId{raw: v}
}
func (oid OrderId) String() string {
	return fmt.Sprintf("%#x", oid.raw)
}
func (oid OrderId) ToUint32() uint32 {
	return uint32(oid.raw)
}
func (oid OrderId) ToUint64() uint64 {
	return oid.raw
}

type Price int

const (
	PriceScale4Dec Price = 10000
	PriceScale2Dec Price = 100
)

func PriceFrom2Dec(price2d int) Price {
	return Price(price2d) * PriceScale4Dec / PriceScale2Dec
}
func PriceFrom4Dec(price4d int) Price {
	return Price(price4d)
}
func PriceTo2Dec(price Price) int {
	return int(price * PriceScale2Dec / PriceScale4Dec)
}
func PriceTo4Dec(price Price) int {
	return int(price)
}

type SecondsMessage interface {
	Seconds() int
}
type ExchangeMessage interface {
	Nanoseconds() int
}
type TradeMessage interface {
	TradeInfo() (OptionId, Price, int)
}
