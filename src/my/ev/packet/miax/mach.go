// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package miax

import (
	"encoding/binary"
	"strconv"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ikravets/errs"

	"my/ev/packet"
)

var EndpointMachSessionMetadata = gopacket.EndpointTypeMetadata{"Mach", func(b []byte) string {
	return strconv.Itoa(int(b[0]))
}}
var EndpointMachSession = gopacket.RegisterEndpointType(10020, EndpointMachSessionMetadata)

// initialized in init() to avoid false detection of potential initialization loop
var LayerTypeMachTop, LayerTypeMach gopacket.LayerType
var MachTopLayerFactory, MachLayerFactory packet.DecodingLayerFactory

func init() {
	LayerTypeMachTop = gopacket.RegisterLayerType(11000, gopacket.LayerTypeMetadata{"MachTop", gopacket.DecodeFunc(decodeMach)})
	LayerTypeMach = gopacket.RegisterLayerType(11001, gopacket.LayerTypeMetadata{"Mach", gopacket.DecodeFunc(decodeMach)})

	MachTopLayerFactory = packet.NewSingleDecodingLayerFactory(
		LayerTypeMachTop,
		func() gopacket.DecodingLayer { return &MachTop{} },
	)
	MachLayerFactory = packet.NewSingleDecodingLayerFactory(
		LayerTypeMach,
		func() gopacket.DecodingLayer { return &Mach{} },
	)
}

type MachTop struct {
	layers.BaseLayer
	MachPackets int
	tps         []packet.TypedPayload
}

var (
	_ packet.DecodingMultiLayer = &MachTop{}
	_ gopacket.Layer            = &MachTop{}
	_ gopacket.DecodingLayer    = &MachTop{}
)

func (m *MachTop) LayerType() gopacket.LayerType {
	return LayerTypeMachTop
}
func (m *MachTop) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) (err error) {
	defer errs.PassE(&err)
	*m = MachTop{
		BaseLayer: layers.BaseLayer{data, data},
		tps:       m.tps[:0], // reuse the slice storage
	}
	for len(data) > 0 {
		errs.Check(len(data) >= 12)
		length := int(binary.LittleEndian.Uint16(data[8:10]))
		errs.Check(length <= len(data), length, len(data))
		m.tps = append(m.tps, packet.TypedPayload{
			Type:    LayerTypeMach,
			Payload: data[:length],
		})
		m.MachPackets++
		data = data[length:]
	}
	return
}
func (m *MachTop) CanDecode() gopacket.LayerClass {
	return LayerTypeMachTop
}
func (m *MachTop) NextLayers() []packet.TypedPayload {
	return m.tps
}
func (m *MachTop) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero // TODO can support chained Mach messages
}

/************************************************************************/
type Mach struct {
	layers.BaseLayer
	SequenceNumber uint64
	Length         uint16
	Type           byte
	SessionNumber  uint8
}

var (
	_ gopacket.Layer         = &Mach{}
	_ gopacket.DecodingLayer = &Mach{}
)

func (m *Mach) LayerType() gopacket.LayerType {
	return LayerTypeMach
}
func (m *Mach) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) (err error) {
	defer errs.PassE(&err)
	errs.Check(len(data) >= 12)
	length := binary.LittleEndian.Uint16(data[8:10])
	errs.Check(len(data) >= int(length))
	*m = Mach{
		SequenceNumber: binary.LittleEndian.Uint64(data[0:8]),
		Length:         length,
		Type:           data[10],
		SessionNumber:  data[11],
		BaseLayer: layers.BaseLayer{
			Contents: data[:12],
			Payload:  data[12:length],
		},
	}
	return nil
}
func (m *Mach) CanDecode() gopacket.LayerClass {
	return LayerTypeMach
}
func (m *Mach) NextLayerType() gopacket.LayerType {
	if len(m.Payload) == 0 {
		return gopacket.LayerTypeZero
	} else {
		tomType := TomMessageType(m.Payload[0])
		return tomType.LayerType()
	}
}
func (m *Mach) Flow() gopacket.Flow {
	session := []byte{m.SessionNumber}
	return gopacket.NewFlow(EndpointMachSession, session, session)
}
func decodeMach(data []byte, p gopacket.PacketBuilder) (err error) {
	panic("FIXME")
	defer errs.PassE(&err)
	m := &Mach{}
	errs.CheckE(m.DecodeFromBytes(data, p))
	p.AddLayer(m)
	return p.NextDecoder(m.NextLayerType())
}
