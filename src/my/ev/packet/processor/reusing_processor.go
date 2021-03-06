// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package processor

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ikravets/errs"

	"my/ev/packet"
	"my/ev/packet/bats"
	"my/ev/packet/miax"
	"my/ev/packet/nasdaq"
)

type reusingProcessor struct {
	obtainer       packet.Obtainer
	handler        packet.Handler
	packetNumLimit int
	pkt            reusingPacket
	m              applicationMessage
	nextSeqNums    []uint64
	messageNum     int
	messageIndex   int
}

// default processor is reusing processor
func NewProcessor() packet.Processor {
	return NewReusingProcessor()
}

func NewReusingProcessor() packet.Processor {
	return &reusingProcessor{
		handler: &packet.NopHandler{},
	}
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
	defer errs.PassE(&err)
	pd := NewUdpDstPortPayloadDetector()
	for port := 30100; port < 30200; port++ {
		pd.addPortMap(layers.UDPPort(port), bats.LayerTypeBSU)
	}
	for port := 51000; port < 51100; port++ {
		pd.addPortMap(layers.UDPPort(port), miax.LayerTypeMachTop)
	}
	for port := 18000; port < 18010; port++ {
		pd.addPortMap(layers.UDPPort(port), nasdaq.LayerTypeMoldUDP64)
	}
	pmlf := &payloadMuxLayerFactory{}
	pmlf.AddDetector(pd)

	parser := packet.NewReusingLayerParser(layers.LayerTypeEthernet)
	parser.AddDecodingLayerFactory(EthernetLayerFactory)
	parser.AddDecodingLayerFactory(Dot1QLayerFactory)
	parser.AddDecodingLayerFactory(IPv4LayerFactory)
	parser.AddDecodingLayerFactory(UDPLayerFactory)
	parser.AddDecodingLayerFactory(TcpIgnoreLayerFactory)
	parser.AddDecodingLayerFactory(pmlf)
	parser.AddDecodingLayerFactory(miax.MachTopLayerFactory)
	parser.AddDecodingLayerFactory(miax.MachLayerFactory)
	parser.AddDecodingLayerFactory(miax.TomLayerFactory)
	parser.AddDecodingLayerFactory(bats.BSULayerFactory)
	parser.AddDecodingLayerFactory(bats.PitchLayerFactory)
	parser.AddDecodingLayerFactory(nasdaq.MoldUDP64LayerFactory)
	parser.AddDecodingLayerFactory(nasdaq.MoldUDP64MessageBlockLayerFactory)
	parser.AddDecodingLayerFactory(nasdaq.IttoLayerFactory)

	packetNumLimit := -1
	if p.packetNumLimit > 0 {
		packetNumLimit = p.packetNumLimit
	}
	var decoded []gopacket.DecodingLayer
	pmlf.SetDecodedLayers(&decoded)
	for packetNum := 0; packetNum != packetNumLimit; packetNum++ {
		//log.Printf("packetNum: %d\n", packetNum)
		data, ci, err := p.obtainer.ZeroCopyReadPacketData()
		if err == io.EOF {
			break
		}
		errs.CheckE(err)
		errs.CheckE(parser.DecodeLayers(data, &decoded))
		errs.CheckE(p.ProcessPacket(data, ci, decoded))
	}
	return
}

func (p *reusingProcessor) ProcessPacket(data []byte, ci gopacket.CaptureInfo, decoded []gopacket.DecodingLayer) (err error) {
	defer errs.PassE(&err)
	p.pkt = reusingPacket{
		data:   data,
		ci:     ci,
		layers: decoded,
	}
	p.m = applicationMessage{
		flows:     p.m.flows[:0],
		timestamp: p.pkt.Timestamp(),
	}
	p.handler.HandlePacket(&p.pkt)
	for _, layer := range decoded {
		//log.Printf("%#v", layer)
		switch l := layer.(type) {
		case gopacket.NetworkLayer:
			p.m.flows = append(p.m.flows, l.NetworkFlow())
		case *layers.UDP:
			p.m.flows = append(p.m.flows, l.TransportFlow())
		case *miax.MachTop:
			p.messageNum = l.MachPackets
			p.messageIndex = 0
			p.nextSeqNums = p.nextSeqNums[:0]
		case *miax.Mach:
			errs.Check(len(p.nextSeqNums) < p.messageNum, len(p.nextSeqNums), p.messageNum)
			p.nextSeqNums = append(p.nextSeqNums, l.SequenceNumber)
		case miax.TomMessage:
			errs.Check(p.messageIndex < p.messageNum, p.messageIndex, p.messageNum)
			p.m.layer = l
			p.m.seqNum = p.nextSeqNums[p.messageIndex]
			p.messageIndex++
			p.handler.HandleMessage(&p.m)
		case *bats.BSU:
			p.m.seqNum = uint64(l.Sequence)
		case bats.PitchMessage:
			p.m.layer = l
			p.handler.HandleMessage(&p.m)
			p.m.seqNum++
		case *nasdaq.MoldUDP64:
			p.m.seqNum = l.SequenceNumber
		case nasdaq.IttoMessage:
			p.m.layer = l
			p.handler.HandleMessage(&p.m)
			p.m.seqNum++
		}
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
	var err error
	_, err = fmt.Fprintf(&b, "PACKET: %d bytes", len(rp.Data()))
	errs.CheckE(err)
	if rp.ci.Length > 0 {
		_, err = fmt.Fprintf(&b, ", wire length %d cap length %d", rp.ci.Length, rp.ci.CaptureLength)
		errs.CheckE(err)
	}
	if !rp.ci.Timestamp.IsZero() {
		_, err = fmt.Fprintf(&b, " @ %v", rp.ci.Timestamp)
		errs.CheckE(err)
	}
	b.WriteByte('\n')
	for i, l := range rp.layers {
		if gl, ok := l.(gopacket.Layer); ok {
			_, err = fmt.Fprintf(&b, "- Layer %d (%02d bytes) = %s\n", i+1, len(gl.LayerContents()), gopacket.LayerString(gl))
		} else {
			_, err = fmt.Fprintf(&b, "- Layer %d <cannot print>\n", i+1)
		}
		errs.CheckE(err)
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
var (
	EthernetLayerFactory = packet.NewSingleDecodingLayerFactory(
		layers.LayerTypeEthernet,
		func() gopacket.DecodingLayer { return &layers.Ethernet{} },
	)
	Dot1QLayerFactory = packet.NewSingleDecodingLayerFactory(
		layers.LayerTypeDot1Q,
		func() gopacket.DecodingLayer { return &layers.Dot1Q{} },
	)
	IPv4LayerFactory = packet.NewSingleDecodingLayerFactory(
		layers.LayerTypeIPv4,
		func() gopacket.DecodingLayer { return &layers.IPv4{} },
	)
	UDPLayerFactory = packet.NewSingleDecodingLayerFactory(
		layers.LayerTypeUDP,
		func() gopacket.DecodingLayer { return &layers.UDP{} },
	)
	TcpIgnoreLayerFactory = packet.NewSingleDecodingLayerFactory(
		layers.LayerTypeTCP,
		func() gopacket.DecodingLayer { return &gopacket.Payload{} },
	)
)

/************************************************************************/
var _ = log.Ldate
