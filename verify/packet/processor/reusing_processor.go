// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package processor

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"time"

	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/layers"

	"my/errs"

	"my/itto/verify/packet"
	"my/itto/verify/packet/miax"
)

type reusingProcessor struct {
	obtainer       packet.Obtainer
	handler        packet.Handler
	packetNumLimit int
	flowBufSrc     bytes.Buffer
	flowBufDst     bytes.Buffer
}

func NewReusingProcessor() packet.Processor {
	return &reusingProcessor{}
}

func (p *reusingProcessor) SetObtainer(o packet.Obtainer) {
	p.obtainer = o
}

func (p *reusingProcessor) SetHandler(handler packet.Handler) {
	p.handler = handler
}

func (p *reusingProcessor) LimitPacketNumber(limit int) {
	p.packetNumLimit = limit
}

func (p *reusingProcessor) ProcessAll() (err error) {
	errs.PassE(&err)
	parser := packet.NewReusingLayerParser(layers.LayerTypeEthernet)
	parser.AddDecodingLayerFactory(EthernetLayerFactory)
	parser.AddDecodingLayerFactory(Dot1QLayerFactory)
	parser.AddDecodingLayerFactory(IPv4LayerFactory)
	parser.AddDecodingLayerFactory(UDPLayerFactory)
	parser.AddDecodingLayerFactory(TcpIgnoreLayerFactory)
	parser.AddDecodingLayerFactory(MachTopLayerFactory)
	parser.AddDecodingLayerFactory(MachLayerFactory)
	parser.AddDecodingLayerFactory(&miax.TomLayerFactory{})

	packetNumLimit := -1
	if p.packetNumLimit > 0 {
		packetNumLimit = p.packetNumLimit
	}
	var decoded []gopacket.DecodingLayer
	for packetNum := 0; packetNum != packetNumLimit; packetNum++ {
		//log.Printf("packetNum: %d\n", packetNum)
		data, ci, err := p.obtainer.ZeroCopyReadPacketData()
		if err == io.EOF {
			break
		}
		errs.CheckE(err)
		//log.Printf("decoding\n")
		//errs.CheckE(parser.DecodeLayers(data, &decoded))
		if err = parser.DecodeLayers(data, &decoded); err != nil {
			log.Printf("decoding error at packet %d: %s\n", packetNum, err)
		}
		//log.Printf("decoded: %#v\n", decoded)
		errs.CheckE(p.ProcessPacket(data, ci, decoded))
	}
	return
}

func (p *reusingProcessor) ProcessPacket(data []byte, ci gopacket.CaptureInfo, decoded []gopacket.DecodingLayer) (err error) {
	errs.PassE(&err)
	pkt, err := p.CreatePacket(data, ci, decoded)
	errs.CheckE(err)
	p.handler.HandlePacket(pkt)
	p.flowBufSrc.Reset()
	p.flowBufDst.Reset()
	var flow gopacket.Flow
	var seqNum uint64
	for _, layer := range decoded {
		//log.Printf("%v", layer)
		continue
		switch l := layer.(type) {
		case gopacket.NetworkLayer:
			p.flowBufSrc.Write(l.NetworkFlow().Src().Raw())
			p.flowBufDst.Write(l.NetworkFlow().Dst().Raw())
		case *layers.UDP:
			p.flowBufSrc.Write(l.TransportFlow().Src().Raw())
			p.flowBufDst.Write(l.TransportFlow().Dst().Raw())
		case *miax.Mach:
			flow = gopacket.NewFlow(packet.EndpointCombinedSession, p.flowBufSrc.Bytes(), p.flowBufDst.Bytes())
			seqNum = l.SequenceNumber
		case miax.TomMessage:
			m := applicationMessage{
				layer:     l,
				flow:      flow,
				seqNum:    seqNum,
				timestamp: pkt.Timestamp(),
			}
			p.handler.HandleMessage(&m)
		}
	}
	return
}

func (p *reusingProcessor) CreatePacket(data []byte, ci gopacket.CaptureInfo, decoded []gopacket.DecodingLayer) (packet packet.Packet, err error) {
	packet = &reusingPacket{
		data:   data,
		ci:     ci,
		layers: decoded,
	}
	return
}

var _ packet.Packet = &reusingPacket{}

type reusingPacket struct {
	data   []byte
	ci     gopacket.CaptureInfo
	layers []gopacket.DecodingLayer
}

func (rp *reusingPacket) String() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "PACKET: %d bytes", len(rp.Data()))
	if rp.ci.Length > 0 {
		fmt.Fprintf(&b, ", wire length %d cap length %d", rp.ci.Length, rp.ci.CaptureLength)
	}
	if !rp.ci.Timestamp.IsZero() {
		fmt.Fprintf(&b, " @ %v", rp.ci.Timestamp)
	}
	b.WriteByte('\n')
	for i, l := range rp.layers {
		if gl, ok := l.(gopacket.Layer); ok {
			fmt.Fprintf(&b, "- Layer %d (%02d bytes) = %s\n", i+1, len(gl.LayerContents()), gopacket.LayerString(gl))
		} else {
			fmt.Fprintf(&b, "- Layer %d <cannot print>\n", i+1)
		}
	}
	return b.String()
}
func (rp *reusingPacket) Data() []byte {
	return rp.data
}
func (rp *reusingPacket) Timestamp() time.Time {
	return rp.ci.Timestamp
}

/************************************************************************/
type SingleDecodingLayerFactory struct {
	layerType gopacket.LayerType
	create    func() gopacket.DecodingLayer
}

var _ packet.DecodingLayerFactory = &SingleDecodingLayerFactory{}

func NewSingleDecodingLayerFactory(layerType gopacket.LayerType, create func() gopacket.DecodingLayer) *SingleDecodingLayerFactory {
	return &SingleDecodingLayerFactory{
		layerType: layerType,
		create:    create,
	}
}
func (f *SingleDecodingLayerFactory) Create(layerType gopacket.LayerType) gopacket.DecodingLayer {
	errs.Check(layerType == f.layerType)
	return f.create()
}
func (f *SingleDecodingLayerFactory) SupportedLayers() gopacket.LayerClass {
	return f.layerType
}

var (
	EthernetLayerFactory = NewSingleDecodingLayerFactory(
		layers.LayerTypeEthernet,
		func() gopacket.DecodingLayer { return &layers.Ethernet{} },
	)
	Dot1QLayerFactory = NewSingleDecodingLayerFactory(
		layers.LayerTypeDot1Q,
		func() gopacket.DecodingLayer { return &layers.Dot1Q{} },
	)
	IPv4LayerFactory = NewSingleDecodingLayerFactory(
		layers.LayerTypeIPv4,
		func() gopacket.DecodingLayer { return &layers.IPv4{} },
	)
	UDPLayerFactory = NewSingleDecodingLayerFactory(
		layers.LayerTypeUDP,
		//func() gopacket.DecodingLayer { return &layers.UDP{} },
		func() gopacket.DecodingLayer { return &MyUdp{} },
	)
	TcpIgnoreLayerFactory = NewSingleDecodingLayerFactory(
		layers.LayerTypeTCP,
		func() gopacket.DecodingLayer { return &gopacket.Payload{} },
	)
	MachTopLayerFactory = NewSingleDecodingLayerFactory(
		miax.LayerTypeMachTop,
		func() gopacket.DecodingLayer { return &miax.MachTop{} },
	)
	MachLayerFactory = NewSingleDecodingLayerFactory(
		miax.LayerTypeMach,
		func() gopacket.DecodingLayer { return &miax.Mach{} },
	)
)

/************************************************************************/
type MyUdp struct {
	layers.UDP
}

func (u *MyUdp) NextLayerType() gopacket.LayerType {
	// FIXME
	return miax.LayerTypeMachTop
}

var _ = log.Ldate
