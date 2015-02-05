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

type MuxObserver struct {
	slaves []Observer
}

func (mo *MuxObserver) AppendSlave(slave Observer) {
	mo.slaves = append(mo.slaves, slave)
}
func (mo *MuxObserver) MessageArrived(m *IttoDbMessage) {
	for _, slave := range mo.slaves {
		slave.MessageArrived(m)
	}
}
func (mo *MuxObserver) OperationAppliedToOrders(o IttoOperation) {
	for _, slave := range mo.slaves {
		slave.OperationAppliedToOrders(o)
	}
}
func (mo *MuxObserver) BeforeBookUpdate(b Book, o IttoOperation) {
	for _, slave := range mo.slaves {
		slave.BeforeBookUpdate(b, o)
	}
}
func (mo *MuxObserver) AfterBookUpdate(b Book, o IttoOperation) {
	for _, slave := range mo.slaves {
		slave.AfterBookUpdate(b, o)
	}
}
