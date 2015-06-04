// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"log"

	"github.com/cznic/b"

	"my/itto/verify/packet"
)

type Book interface {
	ApplyOperation(operation SimOperation)
	GetTop(packet.OptionId, packet.MarketSide, int) []PriceLevel
	NumOptions() int
}

func NewBook() Book {
	return &book{
		options: make(map[packet.OptionId]*optionState),
	}
}

type book struct {
	options map[packet.OptionId]*optionState
}

func (b *book) ApplyOperation(operation SimOperation) {
	oid := operation.GetOptionId()
	if oid.Invalid() {
		return
	}
	os, ok := b.options[oid]
	if !ok {
		os = NewOptionState()
		b.options[oid] = os
	}
	if operation.GetPrice() != 0 {
		os.Side(operation.GetSide()).updateLevel(operation.GetPrice(), operation.GetSizeDelta())
	}
}
func (b *book) GetTop(optionId packet.OptionId, side packet.MarketSide, levels int) []PriceLevel {
	os, ok := b.options[optionId]
	if !ok {
		return nil
	}
	s := os.Side(side)

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
func (b *book) NumOptions() int {
	return len(b.options)
}

type PriceLevel struct {
	Price int
	Size  int
}

func (l *PriceLevel) UpdateSize(delta int) bool {
	l.Size += delta
	if l.Size < 0 {
		log.Fatal("size becomes negative ", l, delta)
	}
	return l.Size != 0
}

type optionState struct {
	bid optionSideState
	ask optionSideState
}

func NewOptionState() *optionState {
	return &optionState{
		bid: NewOptionSideState(packet.MarketSideBid),
		ask: NewOptionSideState(packet.MarketSideAsk),
	}
}

type optionSideState struct {
	levels *b.Tree
}

func (os *optionState) Side(side packet.MarketSide) *optionSideState {
	switch side {
	case packet.MarketSideBid:
		return &os.bid
	case packet.MarketSideAsk:
		return &os.ask
	default:
		log.Fatal("wrong side ", side)
	}
	return nil
}

func NewOptionSideState(side packet.MarketSide) optionSideState {
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
			v = PriceLevel{Price: price}
		}
		write = v.UpdateSize(delta)
		return v, write
	}
	if _, written := s.levels.Put(price, upd); !written {
		s.levels.Delete(price)
	}
}
