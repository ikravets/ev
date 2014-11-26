// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package pcap2log

import (
	"fmt"
	"github.com/kr/pretty"
	"io"
	"log"
)

var _ = pretty.Print

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

func (s *simulator) Printf(format string, vs ...interface{}) {
	if _, err := fmt.Fprintf(s.w, format, vs...); err != nil {
		log.Fatal("output error", err)
	}
}

func (s *simulator) Printfln(format string, vs ...interface{}) {
	s.Printf(format, vs...)
	if _, err := fmt.Fprintln(s.w); err != nil {
		log.Fatal("output error", err)
	}
}

func (s *simulator) outMessageNorm(m *QOMessage, typeChar byte) {
	out := func(name, f string, vs ...interface{}) {
		s.Printf("NORM %s %c ", name, typeChar)
		s.Printfln(f, vs...)
	}
	ord, bid, ask := &m.side1, &m.side1, &m.side2
	if bid.side == MarketSideSell {
		bid, ask = ask, bid
	}
	switch m.typ {
	case MessageTypeUnknown: // ignore
	case MessageTypeQuoteAdd:
		out("QBID", "%08x %08x %08x %08x", m.optionId, bid.refNumDelta, bid.size, bid.price)
		out("QASK", "%08x %08x %08x %08x", m.optionId, ask.refNumDelta, ask.size, ask.price)
	case MessageTypeQuoteReplace:
		out("QBID", "%08x %08x %08x %08x", bid.refNumDelta, bid.origRefNumDelta, bid.size, bid.price)
		out("QASK", "%08x %08x %08x %08x", ask.refNumDelta, ask.origRefNumDelta, ask.size, ask.price)
	case MessageTypeQuoteDelete:
		out("QBID", "%08x", bid.origRefNumDelta)
		out("QASK", "%08x", ask.origRefNumDelta)
	case MessageTypeOrderAdd:
		out("ORDER", "%c %08x %08x %08x %08x", ord.side, m.optionId, ord.refNumDelta, ord.size, ord.price)
	case MessageTypeOrderExecute, MessageTypeOrderExecuteWPrice, MessageTypeOrderCancel:
		out("ORDER", "%08x %08x", ord.origRefNumDelta, ord.size)
	case MessageTypeOrderUpdate:
		out("ORDER", "%08x %08x %08x", ord.origRefNumDelta, ord.size, ord.price)
	case MessageTypeOrderReplace:
		out("ORDER", "%08x %08x %08x %08x", ord.refNumDelta, ord.origRefNumDelta, ord.size, ord.price)
	case MessageTypeOrderDelete:
		out("ORDER", "%08x", ord.origRefNumDelta)
	case MessageTypeBlockOrderDelete:
		for _, r := range m.bssdRefs {
			out("ORDER", "%08x", r)
		}
	default:
		log.Fatalf("Unexpected message type %d in %+v\n", m.typ, m)
	}
}
