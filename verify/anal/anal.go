// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package anal

import (
	"my/errs"
	"sort"

	"my/itto/verify/packet/itto"
	"my/itto/verify/sim"
)

type bookStat struct {
	maxLevels int
}

type Analyzer struct {
	observer  observer
	bookStats map[uint64]*bookStat
}

func NewAnalyzer() *Analyzer {
	a := &Analyzer{
		bookStats: make(map[uint64]*bookStat),
	}
	a.observer.analyzer = a
	return a
}

type observer struct {
	analyzer *Analyzer
}

var _ sim.Observer = &observer{}

func (*observer) MessageArrived(*sim.IttoDbMessage)            {}
func (*observer) OperationAppliedToOrders(sim.IttoOperation)   {}
func (*observer) BeforeBookUpdate(sim.Book, sim.IttoOperation) {}
func (o *observer) AfterBookUpdate(book sim.Book, op sim.IttoOperation) {
	oid := op.GetOptionId()
	if oid.Invalid() {
		return
	}
	bs := o.analyzer.book(oid, op.GetSide())
	//bookSize := len(book.GetTop(oid, op.GetSide(), 0))
	b := book.GetTop(oid, op.GetSide(), 0)
	bookSize := len(b)
	if bs.maxLevels < bookSize {
		//log.Printf("%d %s %d: %v\n", oid, op.GetSide(), bookSize, b)
		bs.maxLevels = bookSize
	}
}

func (a *Analyzer) Observer() sim.Observer {
	return &a.observer
}
func (a *Analyzer) book(oid itto.OptionId, side itto.MarketSide) (bs *bookStat) {
	errs.Check(side == itto.MarketSideBid || side == itto.MarketSideAsk)
	key := uint64(oid) | uint64(side)<<32
	var ok bool
	if bs, ok = a.bookStats[key]; !ok {
		bs = &bookStat{}
		a.bookStats[key] = bs
	}
	return
}

type OptionSide struct {
	Oid  itto.OptionId
	Side itto.MarketSide
}
type BSVal struct {
	Levels       int
	OptionNumber int
	Sample       []OptionSide
}
type BSHist []BSVal

func (a *Analyzer) BookSizeHist() BSHist {
	bsv := make(map[int]BSVal)
	var levels []int
	for k, bs := range a.bookStats {
		v := bsv[bs.maxLevels]
		v.OptionNumber++
		if len(v.Sample) < 10 {
			v.Sample = append(v.Sample, OptionSide{
				Oid:  itto.OptionId(k),
				Side: itto.MarketSide(k >> 32),
			})
		}
		if v.OptionNumber == 1 {
			v.Levels = bs.maxLevels
			levels = append(levels, v.Levels)
		}
		bsv[v.Levels] = v
	}
	sort.Ints(levels)

	var bsh BSHist
	for _, l := range levels {
		bsh = append(bsh, bsv[l])
	}
	return bsh
}
