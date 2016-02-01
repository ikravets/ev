// Copyright (c) Ilia Kravets, 2014-2016. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"errors"
	"fmt"

	"github.com/ikravets/errs"
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

const PriceDefaultDec = 4

func PriceFrom2Dec(price2d int) Price {
	return Price(price2d).Scale(2)
}
func PriceFrom4Dec(price4d int) Price {
	return Price(price4d).Scale(4)
}
func PriceTo2Dec(price Price) int {
	return price.ToInt(2)
}
func PriceTo4Dec(price Price) int {
	return price.ToInt(4)
}
func (p Price) Scale(decimals_stored int) Price {
	mult := []Price{
		1,
		10,
		100,
		1000,
		10000,
		100000,
		1000000,
		10000000,
		100000000,
		1000000000,
		10000000000,
	}
	errs.Check(decimals_stored < len(mult))
	return p * mult[PriceDefaultDec] / mult[decimals_stored]
}
func (p Price) ToInt(decimals int) int {
	mult := []Price{
		1,
		10,
		100,
		1000,
		10000,
		100000,
		1000000,
		10000000,
		100000000,
		1000000000,
		10000000000,
	}
	errs.Check(decimals < len(mult))
	return int(p * mult[decimals] / mult[PriceDefaultDec])
}

type SecondsMessage interface {
	Seconds() int
}
type ExchangeMessage interface {
	Nanoseconds() int
	OptionId() OptionId
}
type TradeMessage interface {
	TradeInfo() (OptionId, Price, int)
}
