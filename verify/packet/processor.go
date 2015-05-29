// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"fmt"
	"time"

	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/layers"
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
