// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package moldudp64

import (
	"encoding/binary"
	"errors"

	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/layers"
)

var EndpointMoldUDP64SessionMetadata = gopacket.EndpointTypeMetadata{"MoldUDP64", func(b []byte) string {
	return string(b[:10])
}}
var EndpointMoldUDP64Session = gopacket.RegisterEndpointType(10000, EndpointMoldUDP64SessionMetadata)

var LayerTypeMoldUDP64 = gopacket.RegisterLayerType(10000, gopacket.LayerTypeMetadata{"MoldUDP64", gopacket.DecodeFunc(decodeMoldUDP64)})

type MoldUDP64 struct {
	layers.BaseLayer
	Session        string
	SequenceNumber uint64
	MessageCount   uint16
}

func (m *MoldUDP64) LayerType() gopacket.LayerType {
	return LayerTypeMoldUDP64
}

func (m *MoldUDP64) Flow() gopacket.Flow {
	session := m.Contents[0:10]
	return gopacket.NewFlow(EndpointMoldUDP64Session, session, session)
}

func decodeMoldUDP64(data []byte, p gopacket.PacketBuilder) error {
	if len(data) < 20 {
		return errors.New("moldUDP64 packet is too short")
	}
	m := &MoldUDP64{
		Session:        string(data[0:10]),
		SequenceNumber: binary.BigEndian.Uint64(data[10:18]),
		MessageCount:   binary.BigEndian.Uint16(data[18:20]),
		BaseLayer:      layers.BaseLayer{data[:20], data[20:]},
	}
	p.AddLayer(m)
	d := MoldUDP64MessageBlockDecoder{parentPacket: m}
	return p.NextDecoder(d)
}

type MoldUDP64MessageBlockDecoder struct {
	parentPacket *MoldUDP64
	//nextOffset   int
}

var LayerTypeMoldUDP64MessageBlock = gopacket.RegisterLayerType(10001, gopacket.LayerTypeMetadata{"MoldUDP64MessageBlock", MoldUDP64MessageBlockDecoder{}})

type MoldUDP64MessageBlock struct {
	MessageLength uint16
	//seqNumber int TODO
	Payload []byte
	tail    []byte
}

func (m *MoldUDP64MessageBlock) LayerType() gopacket.LayerType {
	return LayerTypeMoldUDP64MessageBlock
}

func (m *MoldUDP64MessageBlock) LayerContents() []byte {
	return m.Payload
}

func (m *MoldUDP64MessageBlock) LayerPayload() []byte {
	return m.tail
}

func (d MoldUDP64MessageBlockDecoder) Decode(data []byte, p gopacket.PacketBuilder) error {
	if len(data) < 2 {
		return errors.New("moldUDP64 message block is too short")
	}
	length := binary.BigEndian.Uint16(data[:2])
	m := &MoldUDP64MessageBlock{
		MessageLength: length,
		Payload:       data[2 : length+2],
		tail:          data[length+2:],
	}
	p.AddLayer(m)
	if len(m.tail) == 0 {
		return nil
	}
	nd := MoldUDP64MessageBlockDecoder{parentPacket: d.parentPacket}
	return p.NextDecoder(nd)
}
