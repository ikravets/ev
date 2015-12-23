// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"log"
	"time"

	"my/ev/packet"
	"my/ev/sim"
)

type Stream struct {
	message *sim.SimMessage
	seconds map[int]int
	seqNum  map[int]uint64
}

func NewStream() *Stream {
	l := &Stream{
		seconds: make(map[int]int),
		seqNum:  make(map[int]uint64),
	}
	return l
}
func (l *Stream) MessageArrived(idm *sim.SimMessage) {
	l.message = idm

	idx := l.message.Session.Index()
	seq := l.message.Pam.SequenceNumber()
	if seq != 0 {
		if prevSeq, ok := l.seqNum[idx]; ok && prevSeq+1 != seq {
			log.Printf("seqNum gap; expected %d actual %d\n", prevSeq+1, seq)
		}
		l.seqNum[idx] = seq
	}

	if m, ok := l.message.Pam.Layer().(packet.SecondsMessage); ok {
		l.seconds[idx] = m.Seconds()
	}
}
func (l *Stream) getSeqNum() uint64 {
	idx := l.message.Session.Index()
	return l.seqNum[idx]
}
func (l *Stream) getTimestamp() uint64 {
	idx := l.message.Session.Index()
	return uint64(l.seconds[idx])*1e9 + uint64(l.message.Pam.Layer().(packet.ExchangeMessage).Nanoseconds())
}
func (l *Stream) getExchangeMessage() packet.ExchangeMessage {
	return l.message.Pam.Layer().(packet.ExchangeMessage)
}
func (l *Stream) getPacketTimestamp() time.Time {
	return l.message.Pam.Timestamp()
}
