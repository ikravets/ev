// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

type Observer interface {
	MessageArrived(*SimMessage)
	OperationAppliedToOrders(SimOperation)
	BeforeBookUpdate(Book, SimOperation)
	AfterBookUpdate(Book, SimOperation)
}

type NilObserver struct{}

func (*NilObserver) MessageArrived(*SimMessage)            {}
func (*NilObserver) OperationAppliedToOrders(SimOperation) {}
func (*NilObserver) BeforeBookUpdate(Book, SimOperation)   {}
func (*NilObserver) AfterBookUpdate(Book, SimOperation)    {}

type MuxObserver struct {
	slaves []Observer
}

func (mo *MuxObserver) AppendSlave(slave Observer) {
	mo.slaves = append(mo.slaves, slave)
}
func (mo *MuxObserver) MessageArrived(m *SimMessage) {
	for _, slave := range mo.slaves {
		slave.MessageArrived(m)
	}
}
func (mo *MuxObserver) OperationAppliedToOrders(o SimOperation) {
	for _, slave := range mo.slaves {
		slave.OperationAppliedToOrders(o)
	}
}
func (mo *MuxObserver) BeforeBookUpdate(b Book, o SimOperation) {
	for _, slave := range mo.slaves {
		slave.BeforeBookUpdate(b, o)
	}
}
func (mo *MuxObserver) AfterBookUpdate(b Book, o SimOperation) {
	for _, slave := range mo.slaves {
		slave.AfterBookUpdate(b, o)
	}
}
