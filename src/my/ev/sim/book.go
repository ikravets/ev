// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"log"

	"github.com/cznic/b"

	"my/ev/packet"
)

type PriceLevel interface {
	Price() int
	Size(SizeKind) int
	Equals(PriceLevel) bool
	Clone() PriceLevel
}

type Book interface {
	ApplyOperation(operation SimOperation)
	GetTop(packet.OptionId, packet.MarketSide, int) []PriceLevel
	NumOptions() int
}

func NewBook() Book {
	return &book{
		options:            make(map[packet.OptionId]*optionState),
		newOptionSideState: NewOptionSideStateDeep,
	}
}
func NewBookTop() Book {
	return &book{
		options:            make(map[packet.OptionId]*optionState),
		newOptionSideState: NewOptionSideStateTop,
	}
}

type book struct {
	options            map[packet.OptionId]*optionState
	newOptionSideState func(side packet.MarketSide) optionSideState
}

func (b *book) ApplyOperation(operation SimOperation) {
	oid := operation.GetOptionId()
	if oid.Invalid() {
		return
	}
	os, ok := b.options[oid]
	if !ok {
		os = NewOptionState(b.newOptionSideState)
		b.options[oid] = os
	}
	os.Side(operation.GetSide()).updateLevel(operation)
}
func (b *book) GetTop(optionId packet.OptionId, side packet.MarketSide, levels int) []PriceLevel {
	os, ok := b.options[optionId]
	if !ok {
		return nil
	}
	s := os.Side(side)
	return s.getTop(levels)
}
func (b *book) NumOptions() int {
	return len(b.options)
}

type optionState struct {
	bid optionSideState
	ask optionSideState
}

func NewOptionState(noss func(side packet.MarketSide) optionSideState) *optionState {
	return &optionState{
		bid: noss(packet.MarketSideBid),
		ask: noss(packet.MarketSideAsk),
	}
}

func (os *optionState) Side(side packet.MarketSide) optionSideState {
	switch side {
	case packet.MarketSideBid:
		return os.bid
	case packet.MarketSideAsk:
		return os.ask
	default:
		log.Fatal("wrong side ", side)
	}
	return nil
}

type optionSideState interface {
	updateLevel(operation SimOperation)
	getTop(levels int) []PriceLevel
}

type optionSideStateDeep struct {
	levels *b.Tree
}

func NewOptionSideStateDeep(side packet.MarketSide) optionSideState {
	ComparePrice := func(lhs, rhs interface{}) int {
		l, r := lhs.(int), rhs.(int)
		return l - r
	}
	ComparePriceRev := func(lhs, rhs interface{}) int {
		l, r := lhs.(int), rhs.(int)
		return r - l
	}

	var cmp b.Cmp
	switch side {
	case packet.MarketSideBid:
		cmp = ComparePriceRev
	case packet.MarketSideAsk:
		cmp = ComparePrice
	default:
		log.Fatal("unexpected market side ", side)
	}
	s := &optionSideStateDeep{
		levels: b.TreeNew(cmp),
	}
	return s
}

func (s *optionSideStateDeep) updateLevel(operation SimOperation) {
	price, delta := operation.GetPrice(), operation.GetDefaultSizeDelta()
	if price == 0 {
		return
	}
	upd := func(oldV interface{}, exists bool) (newV interface{}, write bool) {
		var v *priceLevelDefault
		if exists {
			v = oldV.(*priceLevelDefault)
		} else {
			v = newPriceLevelDefault(price)
		}
		write = v.updateSize(delta)
		return v, write
	}
	if _, written := s.levels.Put(price, upd); !written {
		s.levels.Delete(price)
	}
}
func (s *optionSideStateDeep) getTop(levels int) []PriceLevel {
	pl := make([]PriceLevel, 0, levels)
	if it, err := s.levels.SeekFirst(); err == nil {
		for i := 0; i < levels || levels == 0; i++ {
			if _, v, err := it.Next(); err != nil {
				break
			} else {
				pl = append(pl, v.(PriceLevel))
			}
		}
		it.Close()
	}
	return pl
}

type optionSideStateTop struct {
	top priceLevelMulti
}

func NewOptionSideStateTop(side packet.MarketSide) optionSideState {
	return &optionSideStateTop{}
}
func (s *optionSideStateTop) updateLevel(operation SimOperation) {
	s.top.price = operation.GetPrice()
	for i := SizeKindDefault; i < SizeKinds; i++ {
		s.top.sizes[i] = operation.GetNewSize(i)
	}
}
func (s *optionSideStateTop) getTop(levels int) []PriceLevel {
	pl := make([]PriceLevel, 0, 1)
	if s.top != (priceLevelMulti{}) {
		pl = append(pl, &s.top)
	}
	return pl
}

var _ PriceLevel = &priceLevelDefault{}
var _ PriceLevel = &priceLevelMulti{}

type priceLevelDefault struct {
	price int
	size  int
}

func newPriceLevelDefault(price int) *priceLevelDefault {
	return &priceLevelDefault{price: price}
}
func (l *priceLevelDefault) updateSize(delta int) bool {
	l.size += delta
	if l.size < 0 {
		log.Fatal("size becomes negative ", l, delta)
	}
	return l.size != 0
}
func (l *priceLevelDefault) Price() int {
	return l.price
}
func (l *priceLevelDefault) Size(sk SizeKind) int {
	if sk == SizeKindDefault {
		return l.size
	} else {
		return 0
	}
}
func (l *priceLevelDefault) Equals(rhs PriceLevel) (eq bool) {
	if r, ok := rhs.(*priceLevelDefault); ok {
		eq = *l == *r
	}
	return
}
func (l *priceLevelDefault) Clone() PriceLevel {
	return &priceLevelDefault{
		price: l.price,
		size:  l.size,
	}
}

type priceLevelMulti struct {
	price int
	sizes [SizeKinds]int
}

func newPriceLevelMulti(price int) *priceLevelMulti {
	return &priceLevelMulti{price: price}
}
func (l *priceLevelMulti) Price() int {
	return l.price
}
func (l *priceLevelMulti) Size(sk SizeKind) int {
	return l.sizes[sk]
}
func (l *priceLevelMulti) Equals(rhs PriceLevel) (eq bool) {
	if r, ok := rhs.(*priceLevelMulti); ok {
		eq = *l == *r
	}
	return
}
func (l *priceLevelMulti) Clone() PriceLevel {
	return &priceLevelMulti{
		price: l.price,
		sizes: l.sizes,
	}
}

type priceLevelEmpty struct{}

func newpriceLevelEmpty(price int) *priceLevelEmpty {
	return &priceLevelEmpty{}
}
func (l *priceLevelEmpty) Price() int          { return 0 }
func (l *priceLevelEmpty) Size(_ SizeKind) int { return 0 }
func (l *priceLevelEmpty) Clone() PriceLevel   { return l }
func (l *priceLevelEmpty) Equals(rhs PriceLevel) (eq bool) {
	_, eq = rhs.(*priceLevelEmpty)
	return
}

var EmptyPriceLevel PriceLevel = &priceLevelEmpty{}
