// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"log"
	"time"

	"code.google.com/p/gopacket"

	"my/itto/verify/packet/itto"
	"my/itto/verify/sim"
)

type Stream struct {
	message     *sim.IttoDbMessage
	ittoSeconds map[gopacket.Flow]uint32
	seqNum      map[gopacket.Flow]uint64
}

func NewStream() *Stream {
	l := &Stream{
		ittoSeconds: make(map[gopacket.Flow]uint32),
		seqNum:      make(map[gopacket.Flow]uint64),
	}
	return l
}
func (l *Stream) MessageArrived(idm *sim.IttoDbMessage) {
	l.message = idm

	flow := l.message.Pam.Flow()
	seq := l.message.Pam.SequenceNumber()
	if prevSeq, ok := l.seqNum[flow]; ok && prevSeq+1 != seq {
		log.Printf("seqNum gap; expected %d actual %d\n", prevSeq+1, seq)
	}
	l.seqNum[flow] = seq

	switch m := l.message.Pam.Layer().(type) {
	case *itto.IttoMessageSeconds:
		l.ittoSeconds[flow] = m.Second
	case
		*itto.IttoMessageAddOrder,
		*itto.IttoMessageSingleSideExecuted,
		*itto.IttoMessageSingleSideExecutedWithPrice,
		*itto.IttoMessageOrderCancel,
		*itto.IttoMessageSingleSideDelete,
		*itto.IttoMessageBlockSingleSideDelete,
		*itto.IttoMessageSingleSideReplace,
		*itto.IttoMessageSingleSideUpdate,
		*itto.IttoMessageAddQuote,
		*itto.IttoMessageQuoteDelete,
		*itto.IttoMessageQuoteReplace,
		*itto.IttoMessageNoii,
		*itto.IttoMessageOptionsTrade,
		*itto.IttoMessageOptionsCrossTrade,
		*itto.IttoMessageOptionDirectory,
		*itto.IttoMessageOptionOpen,
		*itto.IttoMessageOptionTradingAction:
		// silently ignore
		return
	default:
		log.Println("wrong message type ", idm.Pam.Layer())
		return
	}
}
func (l *Stream) getSeqNum() uint64 {
	flow := l.message.Pam.Flow()
	return l.seqNum[flow]
}
func (l *Stream) getTimestamp() uint64 {
	flow := l.message.Pam.Flow()
	return uint64(l.ittoSeconds[flow])*1e9 + uint64(l.message.Pam.Layer().(itto.IttoMessage).Base().Timestamp)
}
func (l *Stream) getIttoMessage() itto.IttoMessage {
	return l.message.Pam.Layer().(itto.IttoMessage)
}
func (l *Stream) getPacketTimestamp() time.Time {
	return l.message.Pam.Timestamp()
}
