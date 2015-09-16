// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"bufio"
	"fmt"
	"io"

	"github.com/ikravets/errs"

	"my/ev/packet"
)

type Subscr struct {
	subscriptions map[packet.OptionId]struct{}
	stoplist      map[packet.OptionId]struct{}
	autoSubscribe bool
}

func NewSubscr() *Subscr {
	return &Subscr{
		subscriptions: make(map[packet.OptionId]struct{}),
		stoplist:      make(map[packet.OptionId]struct{}),
		autoSubscribe: true,
	}
}
func (s *Subscr) AutoSubscribe(on bool) {
	s.autoSubscribe = on
}
func (s *Subscr) Subscribe(oid packet.OptionId) {
	errs.Check(oid.Valid(), "subscribing to invalid option", oid)
	s.subscriptions[oid] = struct{}{}
	s.autoSubscribe = false
}
func (s *Subscr) Unsubscribe(oid packet.OptionId) {
	errs.Check(oid.Valid(), "unsubscribing from invalid option", oid)
	delete(s.subscriptions, oid)
}
func (s *Subscr) SubscribeFromReader(rd io.Reader) (err error) {
	defer errs.PassE(&err)
	sc := bufio.NewScanner(rd)
	for sc.Scan() {
		text := sc.Text()
		var v uint64
		var b byte
		if _, err := fmt.Sscanf(text, "%c%v", &b, &v); err != nil {
			_, err = fmt.Sscanf(text, "%v", &v)
			errs.CheckE(err)
		}
		oid := packet.OptionIdFromUint64(v)
		if b == 'U' || b == 'u' || b == '!' {
			if s.autoSubscribe {
				s.stoplist[oid] = struct{}{}
			} else {
				s.Unsubscribe(oid)
			}
		} else {
			s.Subscribe(oid)
		}
	}
	errs.CheckE(sc.Err())
	return
}
func (s *Subscr) Subscribed(oid packet.OptionId) bool {
	if s.autoSubscribe {
		_, ok := s.stoplist[oid]
		return !ok
	}
	_, ok := s.subscriptions[oid]
	return ok
}
func (s *Subscr) Num() int {
	if s.autoSubscribe {
		return 0
	}
	num := len(s.subscriptions)
	for k := range s.stoplist {
		if _, ok := s.subscriptions[k]; ok {
			num--
		}
	}
	return num
}
