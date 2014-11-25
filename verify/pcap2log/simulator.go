// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package pcap2log

import (
	"fmt"
	"github.com/kr/pretty"
	"log"
)

func (t *translator) outMessageNorm() {
	m := &t.qom
	ord, bid, ask := &m.side1, &m.side1, &m.side2
	if bid.side == MarketSideSell {
		bid, ask = ask, bid
	}
	switch m.typ {
	case MessageTypeUnknown: // ignore
	case MessageTypeQuoteAdd:
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", t.msgType, m.optionId, bid.refNumDelta, bid.size, bid.price)
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", t.msgType, m.optionId, ask.refNumDelta, ask.size, ask.price)
	case MessageTypeQuoteReplace:
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", t.msgType, bid.refNumDelta, bid.origRefNumDelta, bid.size, bid.price)
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", t.msgType, ask.refNumDelta, ask.origRefNumDelta, ask.size, ask.price)
	case MessageTypeQuoteDelete:
		fmt.Fprintf(t.w, "%s %c %08x\n",
			"NORM QBID", t.msgType, bid.origRefNumDelta)
		fmt.Fprintf(t.w, "%s %c %08x\n",
			"NORM QASK", t.msgType, ask.origRefNumDelta)
	case MessageTypeOrderAdd:
		fmt.Fprintf(t.w, "%s %c %c %08x %08x %08x %08x\n",
			"NORM ORDER", t.msgType, ord.side, m.optionId, ord.refNumDelta, ord.size, ord.price)
	case MessageTypeOrderExecute, MessageTypeOrderExecuteWPrice, MessageTypeOrderCancel:
		fmt.Fprintf(t.w, "%s %c %08x %08x\n",
			"NORM ORDER", t.msgType, ord.origRefNumDelta, ord.size)
	case MessageTypeOrderUpdate:
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x\n",
			"NORM ORDER", t.msgType, ord.origRefNumDelta, ord.size, ord.price)
	case MessageTypeOrderReplace:
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM ORDER", t.msgType, ord.refNumDelta, ord.origRefNumDelta, ord.size, ord.price)
	case MessageTypeOrderDelete:
		fmt.Fprintf(t.w, "%s %c %08x\n",
			"NORM ORDER", t.msgType, ord.origRefNumDelta)
	case MessageTypeBlockOrderDelete:
		for _, r := range m.bssdRefs {
			fmt.Fprintf(t.w, "%s %c %08x\n", "NORM ORDER", t.msgType, r)
		}
	default:
		log.Fatalf("Unexpected message type %d\ntranslator=%s\n", m.typ, pretty.Sprintf("%v", t))
	}
}
