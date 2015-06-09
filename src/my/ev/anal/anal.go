// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package anal

import (
	"sort"

	"my/errs"

	"my/ev/packet"
	"my/ev/sim"
)

type bookStat struct {
	maxLevels int
}

type HashFunc func(uint64) uint64

type orderHashStat struct {
	f             HashFunc
	bucketSize    map[uint64]int
	maxBucketSize map[uint64]int
}
type Analyzer struct {
	observer      observer
	bookStats     map[OptionSide]*bookStat
	orderHashStat []orderHashStat
}

func NewAnalyzer() *Analyzer {
	a := &Analyzer{
		bookStats: make(map[OptionSide]*bookStat),
	}
	a.observer.analyzer = a
	return a
}

type observer struct {
	analyzer *Analyzer
}

var _ sim.Observer = &observer{}

func (*observer) MessageArrived(*sim.SimMessage) {}
func (o *observer) OperationAppliedToOrders(op sim.SimOperation) {
	sess := op.GetMessage().Session.Index()
	errs.Check(sess < 4, sess)
	var key uint64
	var delta int
	switch o := op.(type) {
	case *sim.OperationAdd:
		key = uint64(sess)<<32 | uint64(o.OrderId.ToUint32())
		delta = 1
	case *sim.OperationRemove:
		key = uint64(sess)<<32 | uint64(o.GetOrigOrderId().ToUint32())
		delta = -1
	}
	for _, ohs := range o.analyzer.orderHashStat {
		keyHash := ohs.f(key)
		ohs.bucketSize[keyHash] += delta
		if ohs.bucketSize[keyHash] > ohs.maxBucketSize[keyHash] {
			ohs.maxBucketSize[keyHash] = ohs.bucketSize[keyHash]
		} else if ohs.bucketSize[keyHash] == 0 {
			delete(ohs.bucketSize, keyHash)
		}
	}
}
func (*observer) BeforeBookUpdate(sim.Book, sim.SimOperation) {}
func (o *observer) AfterBookUpdate(book sim.Book, op sim.SimOperation) {
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
func (a *Analyzer) AddOrderHashFunction(f HashFunc) {
	ohs := orderHashStat{
		f:             f,
		bucketSize:    make(map[uint64]int),
		maxBucketSize: make(map[uint64]int),
	}
	a.orderHashStat = append(a.orderHashStat, ohs)
}

func (a *Analyzer) book(oid packet.OptionId, side packet.MarketSide) (bs *bookStat) {
	errs.Check(side == packet.MarketSideBid || side == packet.MarketSideAsk)
	key := OptionSide{
		Oid:  oid,
		Side: side,
	}
	var ok bool
	if bs, ok = a.bookStats[key]; !ok {
		bs = &bookStat{}
		a.bookStats[key] = bs
	}
	return
}

type OptionSide struct {
	Oid  packet.OptionId
	Side packet.MarketSide
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
				Oid:  k.Oid,
				Side: k.Side,
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

type HistVal struct {
	Bin   int
	Count int
}
type Hist []HistVal

func (a *Analyzer) OrdersHashCollisionHist() []Hist {
	var hists []Hist
	for _, ohs := range a.orderHashStat {
		collisionHist := make(map[int]int)
		for _, c := range ohs.maxBucketSize {
			collisionHist[c]++
		}
		var chKeys []int
		for k, _ := range collisionHist {
			chKeys = append(chKeys, k)
		}
		sort.Ints(chKeys)
		var hist Hist
		for _, k := range chKeys {
			hist = append(hist, HistVal{k, collisionHist[k]})
		}
		hists = append(hists, hist)
	}
	return hists
}
