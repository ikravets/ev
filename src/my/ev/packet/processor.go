// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type Obtainer interface {
	gopacket.PacketDataSource
	gopacket.ZeroCopyPacketDataSource
	LinkType() layers.LinkType
}

type ApplicationMessage interface {
	Flow() gopacket.Flow
	Layer() gopacket.Layer
	SequenceNumber() uint64
	Timestamp() time.Time
}

type Handler interface {
	HandlePacket(Packet)
	HandleMessage(ApplicationMessage)
}

type Processor interface {
	SetObtainer(Obtainer)
	SetHandler(Handler)
	LimitPacketNumber(int)
	ProcessAll() error
}

var EndpointCombinedSessionMetadata = gopacket.EndpointTypeMetadata{"Combined", func(b []byte) string {
	return fmt.Sprintf("combined %v", b)
}}
var EndpointCombinedSession = gopacket.RegisterEndpointType(9999, EndpointCombinedSessionMetadata)

type NopHandler struct{}

var _ Handler = &NopHandler{}

func (_ *NopHandler) HandlePacket(_ Packet)              {}
func (_ *NopHandler) HandleMessage(_ ApplicationMessage) {}
