// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package bats

import (
	"encoding/binary"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"my/errs"

	"my/ev/packet"
)

var LayerTypeBSU = gopacket.RegisterLayerType(12000, gopacket.LayerTypeMetadata{"BatsSequencedUnit", nil /*FIXME*/})

type BSU struct {
	layers.BaseLayer
	Length   uint16
	Count    uint8
	Unit     uint8
	Sequence uint32
	tps      []packet.TypedPayload
}

var (
	_ packet.DecodingMultiLayer = &BSU{}
	_ gopacket.Layer            = &BSU{}
	_ gopacket.DecodingLayer    = &BSU{}
)

func (m *BSU) LayerType() gopacket.LayerType {
	return LayerTypeBSU
}
func (m *BSU) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) (err error) {
	errs.PassE(&err)
	*m = BSU{
		Length:    binary.LittleEndian.Uint16(data[0:2]),
		Count:     uint8(data[2]),
		Unit:      uint8(data[3]),
		Sequence:  binary.LittleEndian.Uint32(data[4:8]),
		BaseLayer: layers.BaseLayer{data[:8], data[8:]},
		tps:       m.tps[:0], // reuse the slice storage
	}
	data = m.Payload
	for i := 0; i < int(m.Count); i++ {
		length := int(data[0])
		m.tps = append(m.tps, packet.TypedPayload{
			Type:    PitchMessageType(data[1]).LayerType(),
			Payload: data[:length],
		})
		data = data[length:]
	}
	return
}
func (m *BSU) CanDecode() gopacket.LayerClass {
	return LayerTypeBSU
}
func (m *BSU) NextLayers() []packet.TypedPayload {
	return m.tps
}
func (m *BSU) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero // TODO can support chained Pitch messages
}

var BSULayerFactory = packet.NewSingleDecodingLayerFactory(
	LayerTypeBSU,
	func() gopacket.DecodingLayer { return &BSU{} },
)
