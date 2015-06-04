// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import (
	"bufio"
	"fmt"
	"io"
	"log"

	"my/errs"

	"my/itto/verify/packet"
)

type Subscr struct {
	subscriptions map[packet.OptionId]struct{}
	autoSubscribe bool
}

func NewSubscr() *Subscr {
	return &Subscr{
		subscriptions: make(map[packet.OptionId]struct{}),
		autoSubscribe: true,
	}
}
func (s *Subscr) AutoSubscribe(on bool) {
	s.autoSubscribe = on
}
func (s *Subscr) Subscribe(oid packet.OptionId) {
	if oid.Valid() {
		s.subscriptions[oid] = struct{}{}
	} else {
		log.Fatal("subscribing to invalid option", oid)
	}
	s.autoSubscribe = false
}
func (s *Subscr) SubscribeFromReader(rd io.Reader) (err error) {
	errs.PassE(&err)
	sc := bufio.NewScanner(rd)
	for sc.Scan() {
		text := sc.Text()
		var v uint64
		var b byte
		if _, err := fmt.Sscanf(text, "%c%v", &b, &v); err != nil {
			_, err = fmt.Sscanf(text, "%v", &v)
			errs.CheckE(err)
		}
		s.Subscribe(packet.OptionIdFromUint64(v))
	}
	errs.CheckE(sc.Err())
	return
}
func (s *Subscr) Subscribed(oid packet.OptionId) bool {
	if s.autoSubscribe {
		return true
	}
	_, ok := s.subscriptions[oid]
	return ok
}
