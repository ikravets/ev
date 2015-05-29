// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"time"

	"code.google.com/p/gopacket"
)

// small subset of gopacket.Packet functionality
type Packet interface {
	String() string
	Data() []byte
	Timestamp() time.Time
}

type gopacketWrapper struct {
	gp gopacket.Packet
}

func NewFromGoPacket(gp gopacket.Packet) Packet {
	return &gopacketWrapper{gp: gp}
}
func (gpw *gopacketWrapper) String() string {
	return gpw.gp.String()
}
func (gpw *gopacketWrapper) Data() []byte {
	return gpw.gp.Data()
}
func (gpw *gopacketWrapper) Timestamp() time.Time {
	return gpw.gp.Metadata().Timestamp
}
