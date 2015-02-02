// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"io"
	"log"

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
}

type applicationMessage struct {
	layer gopacket.Layer
	flow  gopacket.Flow
}

func (am *applicationMessage) Layer() gopacket.Layer {
	return am.layer
}

func (am *applicationMessage) Flow() gopacket.Flow {
	return am.flow
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

func (p *processor) ProcessAll() error {
	source := gopacket.NewPacketSource(p.obtainer, p.obtainer.LinkType())
	source.NoCopy = true
	packetNum := 0
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

		var flow gopacket.Flow = gopacket.InvalidFlow
		for _, l := range packet.Layers() {
			if mu, ok := l.(*moldudp64.MoldUDP64); ok {
				if flow != gopacket.InvalidFlow {
					log.Fatal("duplicate MoldUDP64 layer")
				}
				flow = mu.Flow()
			}
			if itto.LayerClassItto.Contains(l.LayerType()) {
				if flow == gopacket.InvalidFlow {
					log.Fatal("incorrect layer order, flow == nil")
				}
				m := applicationMessage{
					layer: l,
					flow:  flow,
				}
				p.handler.HandleMessage(&m)
			}
		}
		packetNum++
		if packetNum == p.packetNumLimit {
			break
		}
	}
	return nil
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
