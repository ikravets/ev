// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"bytes"
	"fmt"
	"io"

	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/layers"

	"my/itto/verify/packet/itto"
	"my/itto/verify/packet/moldudp64"
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
	PacketMetadata() *gopacket.PacketMetadata
}

type applicationMessage struct {
	layer          gopacket.Layer
	flow           gopacket.Flow
	seqNum         uint64
	packetMetadata *gopacket.PacketMetadata
}

func (am *applicationMessage) Layer() gopacket.Layer {
	return am.layer
}
func (am *applicationMessage) Flow() gopacket.Flow {
	return am.flow
}
func (am *applicationMessage) SequenceNumber() uint64 {
	return am.seqNum
}
func (am *applicationMessage) PacketMetadata() *gopacket.PacketMetadata {
	return am.packetMetadata
}

type Handler interface {
	HandlePacket(gopacket.Packet)
	HandleMessage(ApplicationMessage)
}

type Processor interface {
	SetObtainer(Obtainer)
	SetHandler(Handler)
	LimitPacketNumber(int)
	ProcessAll() error
}

type processor struct {
	obtainer       Obtainer
	handler        Handler
	packetNumLimit int
	flowBufSrc     bytes.Buffer
	flowBufDst     bytes.Buffer
}

func NewProcessor() Processor {
	return &processor{}
}

func (p *processor) SetObtainer(o Obtainer) {
	p.obtainer = o
}

func (p *processor) SetHandler(handler Handler) {
	p.handler = handler
}

func (p *processor) LimitPacketNumber(limit int) {
	p.packetNumLimit = limit
}

var EndpointCombinedSessionMetadata = gopacket.EndpointTypeMetadata{"Combined", func(b []byte) string {
	return fmt.Sprintf("combined %v", b)
}}
var EndpointCombinedSession = gopacket.RegisterEndpointType(9999, EndpointCombinedSessionMetadata)

func (p *processor) ProcessAll() error {
	source := gopacket.NewPacketSource(p.obtainer, p.obtainer.LinkType())
	source.NoCopy = true
	packetNum := 0
	var m applicationMessage
	for {
		packet, err := source.NextPacket()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		p.decodeAppLayer(packet) // ignore errors
		p.handler.HandlePacket(packet)
		packetNum++
		if packetNum == p.packetNumLimit {
			break
		}

		mu := moldudp64Layer(packet)
		if mu == nil {
			continue
		}
		seqNum := mu.SequenceNumber
		flow := p.getFlow(packet)
		for _, l := range packet.Layers() {
			if itto.LayerClassItto.Contains(l.LayerType()) {
				m = applicationMessage{
					layer:          l,
					flow:           flow,
					seqNum:         seqNum,
					packetMetadata: packet.Metadata(),
				}
				p.handler.HandleMessage(&m)
				seqNum++
			}
		}
	}
	return nil
}

func (p *processor) getFlow(packet gopacket.Packet) gopacket.Flow {
	mu := moldudp64Layer(packet)
	p.flowBufSrc.Reset()
	p.flowBufDst.Reset()
	p.flowBufSrc.Write(packet.NetworkLayer().NetworkFlow().Src().Raw())
	p.flowBufDst.Write(packet.NetworkLayer().NetworkFlow().Dst().Raw())
	p.flowBufSrc.Write(packet.TransportLayer().TransportFlow().Src().Raw())
	p.flowBufDst.Write(packet.TransportLayer().TransportFlow().Dst().Raw())
	p.flowBufSrc.Write(mu.Flow().Src().Raw())
	p.flowBufDst.Write(mu.Flow().Dst().Raw())
	return gopacket.NewFlow(EndpointCombinedSession, p.flowBufSrc.Bytes(), p.flowBufDst.Bytes())
}

func (p *processor) decodeAppLayer(packet gopacket.Packet) error {
	var moldUdp64Decoder gopacket.Decoder = moldudp64.LayerTypeMoldUDP64
	var ittoDecoder gopacket.Decoder = itto.LayerTypeItto
	//log.Println("decodeAppLayer", packet)
	transpLayer := packet.TransportLayer()
	appLayer := packet.ApplicationLayer()
	if transpLayer == nil || transpLayer.LayerType() != layers.LayerTypeUDP || appLayer == nil {
		return nil
	}

	packetBuilder := packet.(gopacket.PacketBuilder)
	if packetBuilder == nil {
		panic("packet is not packetBuilder")
	}
	if appLayer.LayerType() != gopacket.LayerTypePayload {
		panic("appLayer is not LayerTypePayload")
	}
	data := appLayer.LayerContents()
	if err := moldUdp64Decoder.Decode(data, packetBuilder); err != nil {
		return err
	}
	for _, l := range packet.Layers() {
		if mb, ok := l.(*moldudp64.MoldUDP64MessageBlock); ok {
			ittoDecoder.Decode(mb.Payload, packetBuilder)
		}
	}
	return nil
}

func moldudp64Layer(packet gopacket.Packet) *moldudp64.MoldUDP64 {
	if l := packet.Layer(moldudp64.LayerTypeMoldUDP64); l != nil {
		return l.(*moldudp64.MoldUDP64)
	}
	return nil
}
