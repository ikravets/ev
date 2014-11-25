// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package pcap2log

import (
	"fmt"
	"github.com/kr/pretty"
	"io"
	"log"
)

type simulator struct {
	w io.Writer
}

func NewSimulator(w io.Writer) simulator {
	return simulator{
		w: w,
	}
}

func (s *simulator) addMessage(qom *QOMessage, typeChar byte) {
	s.outMessageNorm(qom, typeChar)
}

func (s *simulator) outMessageNorm(m *QOMessage, typeChar byte) {
	ord, bid, ask := &m.side1, &m.side1, &m.side2
	if bid.side == MarketSideSell {
		bid, ask = ask, bid
	}
	switch m.typ {
	case MessageTypeUnknown: // ignore
	case MessageTypeQuoteAdd:
		fmt.Fprintf(s.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", typeChar, m.optionId, bid.refNumDelta, bid.size, bid.price)
		fmt.Fprintf(s.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", typeChar, m.optionId, ask.refNumDelta, ask.size, ask.price)
	case MessageTypeQuoteReplace:
		fmt.Fprintf(s.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", typeChar, bid.refNumDelta, bid.origRefNumDelta, bid.size, bid.price)
		fmt.Fprintf(s.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", typeChar, ask.refNumDelta, ask.origRefNumDelta, ask.size, ask.price)
	case MessageTypeQuoteDelete:
		fmt.Fprintf(s.w, "%s %c %08x\n",
			"NORM QBID", typeChar, bid.origRefNumDelta)
		fmt.Fprintf(s.w, "%s %c %08x\n",
			"NORM QASK", typeChar, ask.origRefNumDelta)
	case MessageTypeOrderAdd:
		fmt.Fprintf(s.w, "%s %c %c %08x %08x %08x %08x\n",
			"NORM ORDER", typeChar, ord.side, m.optionId, ord.refNumDelta, ord.size, ord.price)
	case MessageTypeOrderExecute, MessageTypeOrderExecuteWPrice, MessageTypeOrderCancel:
		fmt.Fprintf(s.w, "%s %c %08x %08x\n",
			"NORM ORDER", typeChar, ord.origRefNumDelta, ord.size)
	case MessageTypeOrderUpdate:
		fmt.Fprintf(s.w, "%s %c %08x %08x %08x\n",
			"NORM ORDER", typeChar, ord.origRefNumDelta, ord.size, ord.price)
	case MessageTypeOrderReplace:
		fmt.Fprintf(s.w, "%s %c %08x %08x %08x %08x\n",
			"NORM ORDER", typeChar, ord.refNumDelta, ord.origRefNumDelta, ord.size, ord.price)
	case MessageTypeOrderDelete:
		fmt.Fprintf(s.w, "%s %c %08x\n",
			"NORM ORDER", typeChar, ord.origRefNumDelta)
	case MessageTypeBlockOrderDelete:
		for _, r := range m.bssdRefs {
			fmt.Fprintf(s.w, "%s %c %08x\n", "NORM ORDER", typeChar, r)
		}
	default:
		log.Fatalf("Unexpected message type %d\nmessage=%s\n", m.typ, pretty.Sprintf("%v", *m))
	}
}
