// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package processor

import (
	"errors"
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"my/ev/packet"
)

type PayloadDetector interface {
	Detect([]byte, *[]gopacket.DecodingLayer) (gopacket.LayerType, error)
}
type PayloadMux struct {
	data          []byte
	decodedLayers *[]gopacket.DecodingLayer
	detectors     []PayloadDetector
	nextLayer     gopacket.LayerType
}

var _ gopacket.Layer = &PayloadMux{}

func (p *PayloadMux) String() string {
	return fmt.Sprintf("%d byte(s)", len(p.data))
}
func (p *PayloadMux) LayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}
func (p *PayloadMux) LayerContents() []byte {
	return p.data
}
func (p *PayloadMux) LayerPayload() []byte {
	return p.data
}
func (p *PayloadMux) CanDecode() gopacket.LayerClass {
	return gopacket.LayerTypePayload
}
func (p *PayloadMux) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	p.data = data
	p.nextLayer = gopacket.LayerTypeZero
	for _, d := range p.detectors {
		layer, err := d.Detect(data, p.decodedLayers)
		if err == nil {
			p.nextLayer = layer
			return nil
		}
	}
	return nil
}
func (p *PayloadMux) NextLayerType() gopacket.LayerType {
	return p.nextLayer
}

/************************************************************************/

type payloadMuxLayerFactory struct {
	decodedLayers *[]gopacket.DecodingLayer
	detectors     []PayloadDetector
}

var _ packet.DecodingLayerFactory = &payloadMuxLayerFactory{}

func (f *payloadMuxLayerFactory) Create(layerType gopacket.LayerType) gopacket.DecodingLayer {
	return &PayloadMux{
		decodedLayers: f.decodedLayers,
		detectors:     f.detectors,
	}
}
func (f *payloadMuxLayerFactory) SupportedLayers() gopacket.LayerClass {
	return gopacket.LayerTypePayload
}
func (f *payloadMuxLayerFactory) AddDetector(detector PayloadDetector) {
	f.detectors = append(f.detectors, detector)
}
func (f *payloadMuxLayerFactory) SetDecodedLayers(decodedLayers *[]gopacket.DecodingLayer) {
	f.decodedLayers = decodedLayers
}

/************************************************************************/
type EndpointPayloadDetector struct {
	srcEndpointMap map[gopacket.Endpoint]gopacket.LayerType
	dstEndpointMap map[gopacket.Endpoint]gopacket.LayerType
}

var _ PayloadDetector = &EndpointPayloadDetector{}
var EndpointPayloadDetectorFailedError = errors.New("payload detection by endpoint failed")

func NewEndpointPayloadDetector() *EndpointPayloadDetector {
	return &EndpointPayloadDetector{
		srcEndpointMap: make(map[gopacket.Endpoint]gopacket.LayerType),
		dstEndpointMap: make(map[gopacket.Endpoint]gopacket.LayerType),
	}
}
func (d *EndpointPayloadDetector) Detect(payload []byte, decodedLayers *[]gopacket.DecodingLayer) (layer gopacket.LayerType, err error) {
	err = EndpointPayloadDetectorFailedError
	for _, dl := range *decodedLayers {
		switch l := dl.(type) {
		case gopacket.TransportLayer:
			layer, err = d.detectByFlow(payload, l.TransportFlow())
		case gopacket.NetworkLayer:
			layer, err = d.detectByFlow(payload, l.NetworkFlow())
		case gopacket.LinkLayer:
			layer, err = d.detectByFlow(payload, l.LinkFlow())
		}
		if err == nil {
			break
		}
	}
	return
}
func (d *EndpointPayloadDetector) detectByFlow(payload []byte, flow gopacket.Flow) (layer gopacket.LayerType, err error) {
	var ok bool
	if layer, ok = d.srcEndpointMap[flow.Src()]; ok {
		return
	}
	if layer, ok = d.dstEndpointMap[flow.Dst()]; ok {
		return
	}
	return gopacket.LayerTypeZero, EndpointPayloadDetectorFailedError
}
func (d *EndpointPayloadDetector) addSrcMap(src gopacket.Endpoint, lt gopacket.LayerType) {
	d.srcEndpointMap[src] = lt
}
func (d *EndpointPayloadDetector) addDstMap(dst gopacket.Endpoint, lt gopacket.LayerType) {
	d.dstEndpointMap[dst] = lt
}

/************************************************************************/
type UdpDstPortPayloadDetector struct {
	portMap [65536]gopacket.LayerType
}

var _ PayloadDetector = &UdpDstPortPayloadDetector{}
var UdpDstPortPayloadDetectorFailedError = errors.New("payload detection by UdpDstPort failed")

func NewUdpDstPortPayloadDetector() *UdpDstPortPayloadDetector {
	return &UdpDstPortPayloadDetector{}
}
func (d *UdpDstPortPayloadDetector) Detect(payload []byte, decodedLayers *[]gopacket.DecodingLayer) (layer gopacket.LayerType, err error) {
	err = UdpDstPortPayloadDetectorFailedError
	for _, dl := range *decodedLayers {
		if u, ok := dl.(*layers.UDP); ok {
			if layer = d.portMap[u.DstPort]; layer != gopacket.LayerTypeZero {
				err = nil
			}
			break
		}
	}
	return
}
func (d *UdpDstPortPayloadDetector) addPortMap(dst layers.UDPPort, lt gopacket.LayerType) {
	d.portMap[dst] = lt
}

/************************************************************************/
type LayerFunctionPayloadDetector struct {
	detectorFn func(gopacket.DecodingLayer) (gopacket.LayerType, error)
}

var _ PayloadDetector = &LayerFunctionPayloadDetector{}
var LayerFunctionPayloadDetectorFailedError = errors.New("payload detection by LayerFunction failed")

func NewLayerFunctionPayloadDetector(detectorFn func(gopacket.DecodingLayer) (gopacket.LayerType, error)) *LayerFunctionPayloadDetector {
	return &LayerFunctionPayloadDetector{detectorFn: detectorFn}
}
func (d *LayerFunctionPayloadDetector) Detect(payload []byte, decodedLayers *[]gopacket.DecodingLayer) (layer gopacket.LayerType, err error) {
	for _, dl := range *decodedLayers {
		layer, err = d.detectorFn(dl)
		if err == nil {
			return
		}
	}
	return gopacket.LayerTypeZero, LayerFunctionPayloadDetectorFailedError
}
