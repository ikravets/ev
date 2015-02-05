// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

type Observer interface {
	MessageArrived(*IttoDbMessage)
	OperationAppliedToOrders(IttoOperation)
	BeforeBookUpdate(Book, IttoOperation)
	AfterBookUpdate(Book, IttoOperation)
}

type NilObserver struct{}

func (*NilObserver) MessageArrived(*IttoDbMessage)          {}
func (*NilObserver) OperationAppliedToOrders(IttoOperation) {}
func (*NilObserver) BeforeBookUpdate(Book, IttoOperation)   {}
func (*NilObserver) AfterBookUpdate(Book, IttoOperation)    {}
