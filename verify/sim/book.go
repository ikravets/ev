// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"log"

	"github.com/cznic/b"

	"my/itto/verify/packet/itto"
)

type Book interface {
	ApplyOperation(operation IttoOperation)
	GetTop(itto.OptionId, itto.MarketSide, int) []PriceLevel
}

func NewBook() Book {
	return &book{
		options: make(map[itto.OptionId]*optionState),
	}
}

type book struct {
	options map[itto.OptionId]*optionState
}

func (b *book) ApplyOperation(operation IttoOperation) {
	oid := operation.GetOptionId()
	if oid.Invalid() {
		return
	}
	os, ok := b.options[oid]
	if !ok {
		os = NewOptionState()
		b.options[oid] = os
	}
	os.Side(operation.GetSide()).updateLevel(operation.GetPrice(), operation.GetSizeDelta())
}

func (b *book) GetTop(optionId itto.OptionId, side itto.MarketSide, levels int) []PriceLevel {
	os, ok := b.options[optionId]
	if !ok {
		return nil
	}
	s := os.Side(side)

	pl := make([]PriceLevel, 0, levels)
	if it, err := s.levels.SeekFirst(); err == nil {
		for i := 0; i < levels; i++ {
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

type PriceLevel struct {
	price int
	size  int
}

func (l *PriceLevel) UpdateSize(delta int) bool {
	l.size += delta
	if l.size < 0 {
		log.Fatal("size becomes negative ", l, delta)
	}
	return l.size != 0
}

type optionState struct {
	bid optionSideState
	ask optionSideState
}

func NewOptionState() *optionState {
	return &optionState{
		bid: NewOptionSideState(itto.MarketSideBid),
		ask: NewOptionSideState(itto.MarketSideAsk),
	}
}

type optionSideState struct {
	levels *b.Tree
}

func (os *optionState) Side(side itto.MarketSide) *optionSideState {
	switch side {
	case itto.MarketSideBid:
		return &os.bid
	case itto.MarketSideAsk:
		return &os.ask
	default:
		log.Fatal("wrong side ", side)
	}
	return nil
}

func NewOptionSideState(side itto.MarketSide) optionSideState {
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
	case itto.MarketSideBid:
		cmp = ComparePriceRev
	case itto.MarketSideAsk:
		cmp = ComparePrice
	default:
		log.Fatal("unexpected market side ", side)
	}
	s := optionSideState{
		levels: b.TreeNew(cmp),
	}
	return s
}

func (s *optionSideState) updateLevel(price int, delta int) {
	upd := func(oldV interface{}, exists bool) (newV interface{}, write bool) {
		var v PriceLevel
		if exists {
			v = oldV.(PriceLevel)
		} else {
			v = PriceLevel{price: price}
		}
		write = v.UpdateSize(delta)
		return v, write
	}
	if _, written := s.levels.Put(price, upd); !written {
		s.levels.Delete(price)
	}
}
