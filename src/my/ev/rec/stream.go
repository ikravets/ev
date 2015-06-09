// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"log"
	"time"

	"github.com/google/gopacket"

	"my/ev/packet"
	"my/ev/sim"
)

type Stream struct {
	message *sim.SimMessage
	seconds map[gopacket.Flow]int
	seqNum  map[gopacket.Flow]uint64
}

func NewStream() *Stream {
	l := &Stream{
		seconds: make(map[gopacket.Flow]int),
		seqNum:  make(map[gopacket.Flow]uint64),
	}
	return l
}
func (l *Stream) MessageArrived(idm *sim.SimMessage) {
	l.message = idm

	flow := l.message.Pam.Flow()
	seq := l.message.Pam.SequenceNumber()
	if seq != 0 {
		if prevSeq, ok := l.seqNum[flow]; ok && prevSeq+1 != seq {
			log.Printf("seqNum gap; expected %d actual %d\n", prevSeq+1, seq)
		}
		l.seqNum[flow] = seq
	}

	if m, ok := l.message.Pam.Layer().(packet.SecondsMessage); ok {
		l.seconds[flow] = m.Seconds()
	}
}
func (l *Stream) getSeqNum() uint64 {
	flow := l.message.Pam.Flow()
	return l.seqNum[flow]
}
func (l *Stream) getTimestamp() uint64 {
	flow := l.message.Pam.Flow()
	return uint64(l.seconds[flow])*1e9 + uint64(l.message.Pam.Layer().(packet.ExchangeMessage).Nanoseconds())
}
func (l *Stream) getExchangeMessage() packet.ExchangeMessage {
	return l.message.Pam.Layer().(packet.ExchangeMessage)
}
func (l *Stream) getPacketTimestamp() time.Time {
	return l.message.Pam.Timestamp()
}
