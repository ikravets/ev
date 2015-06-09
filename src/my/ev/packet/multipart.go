// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import "github.com/google/gopacket"

type DecodingMultiLayer interface {
	//similarly to gopacket.DecodingLayer
	DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error
	CanDecode() gopacket.LayerClass

	NextLayers() []TypedPayload
}

type TypedPayload struct {
	Type    gopacket.LayerType
	Payload []byte
}
