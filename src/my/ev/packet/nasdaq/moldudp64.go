// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package nasdaq

import (
	"encoding/binary"
	"errors"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"my/errs"

	"my/ev/packet"
)

var EndpointMoldUDP64SessionMetadata = gopacket.EndpointTypeMetadata{"MoldUDP64", func(b []byte) string {
	return string(b[:10])
}}
var EndpointMoldUDP64Session = gopacket.RegisterEndpointType(10000, EndpointMoldUDP64SessionMetadata)

var LayerTypeMoldUDP64 = gopacket.RegisterLayerType(10000, gopacket.LayerTypeMetadata{"MoldUDP64", gopacket.DecodeFunc(decodeMoldUDP64)})

// must initialize in init() to avoid false detection of potential initialization loop
var LayerTypeMoldUDP64MessageBlock gopacket.LayerType
var LayerTypeMoldUDP64MessageBlockChained gopacket.LayerType

var MoldUDP64LayerFactory, MoldUDP64MessageBlockLayerFactory packet.DecodingLayerFactory

func init() {
	LayerTypeMoldUDP64MessageBlock = gopacket.RegisterLayerType(10001, gopacket.LayerTypeMetadata{"MoldUDP64MessageBlock", gopacket.DecodeFunc(decodeMoldUDP64MessageBlock)})

	LayerTypeMoldUDP64MessageBlockChained = gopacket.RegisterLayerType(10003, gopacket.LayerTypeMetadata{"MoldUDP64MessageBlockChained", gopacket.DecodeFunc(decodeMoldUDP64MessageBlockChained)})

	MoldUDP64LayerFactory = packet.NewSingleDecodingLayerFactory(
		LayerTypeMoldUDP64,
		func() gopacket.DecodingLayer { return &MoldUDP64{} },
	)

	MoldUDP64MessageBlockLayerFactory = packet.NewSingleDecodingLayerFactory(
		LayerTypeMoldUDP64MessageBlock,
		func() gopacket.DecodingLayer { return &MoldUDP64MessageBlock{} },
	)
}

/************************************************************************/
type MoldUDP64 struct {
	layers.BaseLayer
	Session        string
	SequenceNumber uint64
	MessageCount   uint16
	tps            []packet.TypedPayload
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
		tps:            m.tps[:0], // reuse the slice storage
	}
	data = m.Payload
	for i := 0; i < int(m.MessageCount); i++ {
		length := binary.BigEndian.Uint16(data[0:2]) + 2
		m.tps = append(m.tps, packet.TypedPayload{
			Type:    LayerTypeMoldUDP64MessageBlock,
			Payload: data[:length],
		})
		data = data[length:]
	}
	return nil
}
func (m *MoldUDP64) CanDecode() gopacket.LayerClass {
	return LayerTypeMoldUDP64
}
func (m *MoldUDP64) NextLayers() []packet.TypedPayload {
	return m.tps
}
func (m *MoldUDP64) NextLayerType() gopacket.LayerType {
	return LayerTypeMoldUDP64MessageBlockChained
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

func (m *MoldUDP64) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	errs.PassE(&err)
	bytes, err := b.PrependBytes(20)
	errs.CheckE(err)
	copy(bytes[0:10], m.Session[0:10])
	binary.BigEndian.PutUint64(bytes[10:18], m.SequenceNumber)
	binary.BigEndian.PutUint16(bytes[18:20], m.MessageCount)
	return
}

/************************************************************************/
type MoldUDP64MessageBlock struct {
	layers.BaseLayer
	MessageLength uint16
}

var (
	_ gopacket.Layer         = &MoldUDP64MessageBlock{}
	_ gopacket.DecodingLayer = &MoldUDP64MessageBlock{}
)

func (m *MoldUDP64MessageBlock) LayerType() gopacket.LayerType {
	return LayerTypeMoldUDP64MessageBlock
}

func (m *MoldUDP64MessageBlock) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 2 {
		return errors.New("moldUDP64 message block is too short")
	}
	length := binary.BigEndian.Uint16(data[:2])
	*m = MoldUDP64MessageBlock{
		MessageLength: length,
		BaseLayer:     layers.BaseLayer{data[:2], data[2:]},
	}
	return nil
}

func (m *MoldUDP64MessageBlock) CanDecode() gopacket.LayerClass {
	return LayerTypeMoldUDP64MessageBlock
}

func (m *MoldUDP64MessageBlock) NextLayerType() gopacket.LayerType {
	return IttoMessageType(m.Payload[0]).LayerType()
}

func decodeMoldUDP64MessageBlock(data []byte, p gopacket.PacketBuilder) error {
	m := &MoldUDP64MessageBlock{}
	if err := m.DecodeFromBytes(data, p); err != nil {
		return err
	}
	p.AddLayer(m)
	return p.NextDecoder(m.NextLayerType())
}

/************************************************************************/
type MoldUDP64MessageBlockChained struct {
	MessageLength uint16
	//seqNumber int TODO
	Payload []byte
	tail    []byte
}

var (
	_ gopacket.Layer         = &MoldUDP64MessageBlockChained{}
	_ gopacket.DecodingLayer = &MoldUDP64MessageBlockChained{}
)

func (m *MoldUDP64MessageBlockChained) LayerType() gopacket.LayerType {
	return LayerTypeMoldUDP64MessageBlockChained
}

func (m *MoldUDP64MessageBlockChained) LayerContents() []byte {
	return m.Payload
}

func (m *MoldUDP64MessageBlockChained) LayerPayload() []byte {
	return m.tail
}

func (m *MoldUDP64MessageBlockChained) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 2 {
		return errors.New("moldUDP64 message block is too short")
	}
	length := binary.BigEndian.Uint16(data[:2])
	*m = MoldUDP64MessageBlockChained{
		MessageLength: length,
		Payload:       data[2 : length+2],
		tail:          data[length+2:],
	}
	return nil
}

func (m *MoldUDP64MessageBlockChained) CanDecode() gopacket.LayerClass {
	return LayerTypeMoldUDP64MessageBlockChained
}

func (m *MoldUDP64MessageBlockChained) NextLayerType() gopacket.LayerType {
	if len(m.tail) == 0 {
		return gopacket.LayerTypeZero
	} else {
		return LayerTypeMoldUDP64MessageBlockChained
	}
}

func decodeMoldUDP64MessageBlockChained(data []byte, p gopacket.PacketBuilder) error {
	m := &MoldUDP64MessageBlockChained{}
	if err := m.DecodeFromBytes(data, p); err != nil {
		return err
	}
	p.AddLayer(m)
	return p.NextDecoder(m.NextLayerType())
}

func (m *MoldUDP64MessageBlockChained) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	errs.PassE(&err)
	payload := b.Bytes()
	bytes, err := b.PrependBytes(2)
	errs.CheckE(err)
	if opts.FixLengths {
		m.MessageLength = uint16(len(payload))
	}
	binary.BigEndian.PutUint16(bytes, uint16(m.MessageLength))
	return
}
