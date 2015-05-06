// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"log"
	"my/itto/verify/sim"
)

type HwLimChecker struct {
}

var _ sim.Observer = &HwLimChecker{}

const supernodeLevels = 256
const supernodes = 256 * 1024

func NewHwLimChecker() *HwLimChecker {
	return &HwLimChecker{}
}
func (hlc *HwLimChecker) MessageArrived(*sim.IttoDbMessage)            {}
func (hlc *HwLimChecker) OperationAppliedToOrders(sim.IttoOperation)   {}
func (hlc *HwLimChecker) BeforeBookUpdate(sim.Book, sim.IttoOperation) {}
func (hlc *HwLimChecker) AfterBookUpdate(book sim.Book, operation sim.IttoOperation) {
	opa, ok := operation.(*sim.OperationAdd)
	if !ok || operation.GetOptionId().Invalid() {
		return
	}
	if opa.Independent() {
		if book.NumOptions() == supernodes {
			log.Fatalf("reached hw supernodes limit (%d)\n", supernodes)
		}
	}
	tob := book.GetTop(operation.GetOptionId(), operation.GetSide(), 0)
	if len(tob) > supernodeLevels {
		log.Fatalf("book (oid %d, side %s) has %d levels (>%d)",
			operation.GetOptionId(), operation.GetSide(),
			len(tob), supernodeLevels)
	}
}
