// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"code.google.com/p/gopacket"
)

func (d *db) getSession(flow gopacket.Flow) Session {
	for _, s := range d.sessions {
		if s.flow == flow {
			return s
		}
	}
	s := Session{
		flow:  flow,
		index: len(d.sessions),
	}
	d.sessions = append(d.sessions, s)
	return s
}

type Session struct {
	flow  gopacket.Flow
	index int
}

func (s *Session) Index() int {
	return s.index
}

func (d *db) SetSubscription(s *Subscr) {
	d.subscr = s
}
