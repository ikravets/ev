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

var (
	_ gopacket.Layer         = &MoldUDP64{}
	_ gopacket.DecodingLayer = &MoldUDP64{}
)

func (m *MoldUDP64) LayerType() gopacket.LayerType {
	return LayerTypeMoldUDP64
}

func (m *MoldUDP64) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 20 {
		return errors.New("moldUDP64 packet is too short")
	}
	*m = MoldUDP64{
		Session:        string(data[0:10]),
		SequenceNumber: binary.BigEndian.Uint64(data[10:18]),
		MessageCount:   binary.BigEndian.Uint16(data[18:20]),
		BaseLayer:      layers.BaseLayer{data[:20], data[20:]},
	}
	return nil
}

func (m *MoldUDP64) CanDecode() gopacket.LayerClass {
	return LayerTypeMoldUDP64
}

func (m *MoldUDP64) NextLayerType() gopacket.LayerType {
	return LayerTypeMoldUDP64MessageBlock
}

func (m *MoldUDP64) Flow() gopacket.Flow {
	session := m.Contents[0:10]
	return gopacket.NewFlow(EndpointMoldUDP64Session, session, session)
}

func decodeMoldUDP64(data []byte, p gopacket.PacketBuilder) error {
	m := &MoldUDP64{}
	if err := m.DecodeFromBytes(data, p); err != nil {
		return err
	}
	p.AddLayer(m)
	return p.NextDecoder(m.NextLayerType())
}

// initialized in init() to avoid false detection of potential initialization loop
var LayerTypeMoldUDP64MessageBlock gopacket.LayerType

func init() {
	LayerTypeMoldUDP64MessageBlock = gopacket.RegisterLayerType(10001, gopacket.LayerTypeMetadata{"MoldUDP64MessageBlock", gopacket.DecodeFunc(decodeMoldUDP64MessageBlock)})
}

type MoldUDP64MessageBlock struct {
	MessageLength uint16
	//seqNumber int TODO
	Payload []byte
	tail    []byte
}

var (
	_ gopacket.Layer         = &MoldUDP64MessageBlock{}
	_ gopacket.DecodingLayer = &MoldUDP64MessageBlock{}
)

func (m *MoldUDP64MessageBlock) LayerType() gopacket.LayerType {
	return LayerTypeMoldUDP64MessageBlock
}

func (m *MoldUDP64MessageBlock) LayerContents() []byte {
	return m.Payload
}

func (m *MoldUDP64MessageBlock) LayerPayload() []byte {
	return m.tail
}

func (m *MoldUDP64MessageBlock) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 2 {
		return errors.New("moldUDP64 message block is too short")
	}
	length := binary.BigEndian.Uint16(data[:2])
	*m = MoldUDP64MessageBlock{
		MessageLength: length,
		Payload:       data[2 : length+2],
		tail:          data[length+2:],
	}
	return nil
}

func (m *MoldUDP64MessageBlock) CanDecode() gopacket.LayerClass {
	return LayerTypeMoldUDP64MessageBlock
}

func (m *MoldUDP64MessageBlock) NextLayerType() gopacket.LayerType {
	if len(m.tail) == 0 {
		return gopacket.LayerTypeZero
	} else {
		return LayerTypeMoldUDP64MessageBlock
	}
}

func decodeMoldUDP64MessageBlock(data []byte, p gopacket.PacketBuilder) error {
	m := &MoldUDP64MessageBlock{}
	if err := m.DecodeFromBytes(data, p); err != nil {
		return err
	}
	p.AddLayer(m)
	return p.NextDecoder(m.NextLayerType())
}
