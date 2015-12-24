// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package processor

import (
	"time"

	"github.com/google/gopacket"
)

type applicationMessage struct {
	layer     gopacket.Layer
	flows     []gopacket.Flow
	seqNum    uint64
	timestamp time.Time
}

func (am *applicationMessage) Layer() gopacket.Layer {
	return am.layer
}
func (am *applicationMessage) Flows() []gopacket.Flow {
	return am.flows
}
func (am *applicationMessage) SequenceNumber() uint64 {
	return am.seqNum
}
func (am *applicationMessage) Timestamp() time.Time {
	return am.timestamp
}
