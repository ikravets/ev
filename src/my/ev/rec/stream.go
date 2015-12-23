// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"log"
	"time"

	"github.com/ikravets/errs"

	"my/ev/packet"
	"my/ev/sim"
)

type Stream struct {
	message *sim.SimMessage
	seconds []int
	seqNum  []uint64
}

func NewStream() *Stream {
	return &Stream{}
}
func (l *Stream) MessageArrived(idm *sim.SimMessage) {
	l.message = idm

	idx := l.message.Session.Index()
	if idx >= len(l.seconds) {
		secondsOld := l.seconds
		l.seconds = make([]int, idx+1)
		copy(l.seconds, secondsOld)
		seqNumOld := l.seqNum
		l.seqNum = make([]uint64, idx+1)
		copy(l.seqNum, seqNumOld)
	}
	seq := l.message.Pam.SequenceNumber()
	if seq != 0 {
		if prevSeq := l.seqNum[idx]; prevSeq != 0 && prevSeq+1 != seq {
			log.Printf("seqNum gap; expected %d actual %d\n", prevSeq+1, seq)
		}
		l.seqNum[idx] = seq
	}

	if m, ok := l.message.Pam.Layer().(packet.SecondsMessage); ok {
		l.seconds[idx] = m.Seconds()
	}
}
func (l *Stream) getGroup() uint8 {
	idx := l.message.Session.Index()
	errs.Check(idx >= 0 && idx < 256)
	return uint8(idx)
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
