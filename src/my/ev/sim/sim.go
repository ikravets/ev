// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"github.com/google/gopacket"

	"my/ev/packet"
)

type Sim interface {
	Session(flows []gopacket.Flow) Session
	Subscr() *Subscr
	Options() Options
	OrderDb() OrderDb
	Book() Book
	Sessions() []Session
	SessionsIgnoreSrc(ignore bool)
	NewMessage(packet.ApplicationMessage) *SimMessage
}

type simu struct {
	subscr    *Subscr
	options   Options
	orderDb   OrderDb
	book      Book
	sessions  []Session
	ignoreSrc bool
}

func NewSim(shallow bool) Sim {
	sim := &simu{
		subscr:  NewSubscr(),
		options: NewOptions(),
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
func (sim *simu) Options() Options {
	return sim.options
}
func (sim *simu) Book() Book {
	return sim.book
}
func (sim *simu) OrderDb() OrderDb {
	return sim.orderDb
}
func (sim *simu) SessionsIgnoreSrc(ignore bool) {
	sim.ignoreSrc = ignore
}
func (sim *simu) Sessions() []Session {
	return sim.sessions
}
func (sim *simu) NewMessage(pam packet.ApplicationMessage) *SimMessage {
	return NewSimMessage(sim, pam)
}

func (sim *simu) Session(flows []gopacket.Flow) Session {
	for _, s := range sim.sessions {
		if len(s.flows) != len(flows) {
			continue
		}
		match := true
		for i := range s.flows {
			if sim.ignoreSrc {
				match = match && s.flows[i].Dst() == flows[i].Dst()
			} else {
				match = match && s.flows[i] == flows[i]
			}
			if !match {
				break
			}
		}
		if match {
			return s
		}
	}
	s := Session{
		flows: make([]gopacket.Flow, len(flows)),
		index: len(sim.sessions),
	}
	copy(s.flows, flows)
	sim.sessions = append(sim.sessions, s)
	return s
}

type Session struct {
	flows []gopacket.Flow
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
