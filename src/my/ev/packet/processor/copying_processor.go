// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package processor

import (
	"bytes"
	"io"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"my/ev/packet"
	"my/ev/packet/nasdaq"
)

type processor struct {
	obtainer       packet.Obtainer
	handler        packet.Handler
	packetNumLimit int
	flowBufSrc     bytes.Buffer
	flowBufDst     bytes.Buffer
}

func NewCopyingProcessor() packet.Processor {
	return &processor{
		handler: &packet.NopHandler{},
	}
}

func (p *processor) SetObtainer(o packet.Obtainer) {
	p.obtainer = o
}

func (p *processor) SetHandler(handler packet.Handler) {
	p.handler = handler
}

func (p *processor) LimitPacketNumber(limit int) {
	p.packetNumLimit = limit
}

func (p *processor) ProcessAll() error {
	source := gopacket.NewPacketSource(p.obtainer, p.obtainer.LinkType())
	source.NoCopy = true
	packetNum := 0
	var m applicationMessage
	for {
		pkt, err := source.NextPacket()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		p.decodeAppLayer(pkt) // ignore errors
		p.handler.HandlePacket(packet.NewFromGoPacket(pkt))
		packetNum++
		if packetNum == p.packetNumLimit {
			break
		}

		mu := moldudp64Layer(pkt)
		if mu == nil {
			continue
		}
		seqNum := mu.SequenceNumber
		flow := p.getFlow(pkt)
		for _, l := range pkt.Layers() {
			if nasdaq.LayerClassItto.Contains(l.LayerType()) {
				m = applicationMessage{
					layer:     l,
					flow:      flow,
					seqNum:    seqNum,
					timestamp: pkt.Metadata().Timestamp,
				}
				p.handler.HandleMessage(&m)
				seqNum++
			}
		}
	}
	return nil
}

func (p *processor) getFlow(pkt gopacket.Packet) gopacket.Flow {
	mu := moldudp64Layer(pkt)
	//p.flowBufSrc.Reset()
	//p.flowBufSrc.Write(pkt.NetworkLayer().NetworkFlow().Src().Raw())
	//p.flowBufSrc.Write(pkt.TransportLayer().TransportFlow().Src().Raw())
	//p.flowBufSrc.Write(mu.Flow().Src().Raw())
	p.flowBufDst.Reset()
	p.flowBufDst.Write(pkt.NetworkLayer().NetworkFlow().Dst().Raw())
	p.flowBufDst.Write(pkt.TransportLayer().TransportFlow().Dst().Raw())
	p.flowBufDst.Write(mu.Flow().Dst().Raw())
	return gopacket.NewFlow(packet.EndpointCombinedSession, p.flowBufSrc.Bytes(), p.flowBufDst.Bytes())
}

func (p *processor) decodeAppLayer(pkt gopacket.Packet) error {
	var moldUdp64Decoder gopacket.Decoder = nasdaq.LayerTypeMoldUDP64
	var ittoDecoder gopacket.Decoder = nasdaq.LayerTypeItto
	//log.Println("decodeAppLayer", pkt)
	transpLayer := pkt.TransportLayer()
	appLayer := pkt.ApplicationLayer()
	if transpLayer == nil || transpLayer.LayerType() != layers.LayerTypeUDP || appLayer == nil {
		return nil
	}

	packetBuilder := pkt.(gopacket.PacketBuilder)
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
	for _, l := range pkt.Layers() {
		if mb, ok := l.(*nasdaq.MoldUDP64MessageBlockChained); ok {
			ittoDecoder.Decode(mb.Payload, packetBuilder)
		}
	}
	return nil
}

func moldudp64Layer(pkt gopacket.Packet) *nasdaq.MoldUDP64 {
	if l := pkt.Layer(nasdaq.LayerTypeMoldUDP64); l != nil {
		return l.(*nasdaq.MoldUDP64)
	}
	return nil
}
