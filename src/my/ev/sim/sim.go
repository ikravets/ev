// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"github.com/google/gopacket"

	"my/ev/packet"
)

type Sim interface {
	Session(flow gopacket.Flow) Session
	Subscr() *Subscr
	OrderDb() OrderDb
	Book() Book
	Sessions() []Session
	NewMessage(packet.ApplicationMessage) *SimMessage
}

type simu struct {
	subscr   *Subscr
	orderDb  OrderDb
	book     Book
	sessions []Session
}

func NewSim(shallow bool) Sim {
	sim := &simu{
		subscr: NewSubscr(),
	}
	if shallow {
		sim.book = NewBookTop()
	} else {
		sim.book = NewBook()
	}
	sim.orderDb = NewOrderDb(sim)
	return sim
}
func (sim *simu) Subscr() *Subscr {
	return sim.subscr
}
func (sim *simu) Book() Book {
	return sim.book
}
func (sim *simu) OrderDb() OrderDb {
	return sim.orderDb
}
func (sim *simu) Sessions() []Session {
	return sim.sessions
}
func (sim *simu) NewMessage(pam packet.ApplicationMessage) *SimMessage {
	return NewSimMessage(sim, pam)
}

func (sim *simu) Session(flow gopacket.Flow) Session {
	for _, s := range sim.sessions {
		if s.flow == flow {
			return s
		}
	}
	s := Session{
		flow:  flow,
		index: len(sim.sessions),
	}
	sim.sessions = append(sim.sessions, s)
	return s
}

type Session struct {
	flow  gopacket.Flow
	index int
}

func (s *Session) Index() int {
	return s.index
}

// a mix of OrderCapacity and ExecInst (in FIX terms)
type SizeKind int

const (
	SizeKindDefault SizeKind = iota
	SizeKindAON
	SizeKindCustomer
	SizeKindCustomerAON
	SizeKinds
)
