// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"log"

	"github.com/google/gopacket"

	"my/errs"
)

type DecodingLayerFactory interface {
	Create(gopacket.LayerType) gopacket.DecodingLayer
	SupportedLayers() gopacket.LayerClass
}

type ReusingLayerParser struct {
	first      gopacket.LayerType
	factories  map[gopacket.LayerType]DecodingLayerFactory
	layerCache map[gopacket.LayerType][]gopacket.DecodingLayer
	df         gopacket.DecodeFeedback
	Truncated  bool
}

func NewReusingLayerParser(first gopacket.LayerType, factories ...DecodingLayerFactory) *ReusingLayerParser {
	p := &ReusingLayerParser{
		factories:  make(map[gopacket.LayerType]DecodingLayerFactory),
		layerCache: make(map[gopacket.LayerType][]gopacket.DecodingLayer),
		first:      first,
	}
	p.df = p // cast this once to the interface
	for _, f := range factories {
		p.AddDecodingLayerFactory(f)
	}
	p.AddDecodingLayerFactory(UnknownDecodingLayerFactory)
	return p
}

func (p *ReusingLayerParser) AddDecodingLayerFactory(f DecodingLayerFactory) {
	for _, typ := range f.SupportedLayers().LayerTypes() {
		if p.factories[typ] == nil {
			p.factories[typ] = f
		}
	}
}

func (p *ReusingLayerParser) DecodeLayers(data []byte, decoded *[]gopacket.DecodingLayer) (err error) {
	errs.PassE(&err)
	p.recycle(*decoded)
	*decoded = (*decoded)[:0]
	p.Truncated = false

	tps := []TypedPayload{TypedPayload{Type: p.first, Payload: data}}
	for len(tps) > 0 {
		tp := tps[0]
		tps = tps[1:]
		if tp.Type == gopacket.LayerTypeZero {
			continue
		}
		layer, err := p.tryDecode(tp)
		if err != nil {
			log.Printf("error: %s\n", err)
			tp.Type = gopacket.LayerTypeDecodeFailure
			layer, err = p.tryDecode(tp)
			errs.CheckE(err)
		}
		*decoded = append(*decoded, layer)

		if dml, ok := layer.(DecodingMultiLayer); ok {
			tps = append(tps, dml.NextLayers()...)
		} else if len(layer.LayerPayload()) > 0 {
			tps = append(tps, TypedPayload{Type: layer.NextLayerType(), Payload: layer.LayerPayload()})
		}
	}
	return nil
}

func (p *ReusingLayerParser) tryDecode(tp TypedPayload) (layer gopacket.DecodingLayer, err error) {
	if layer, err = p.getDecodingLayer(tp.Type); err != nil {
		return
	}
	if err = layer.DecodeFromBytes(tp.Payload, p.df); err != nil {
		p.recycle([]gopacket.DecodingLayer{layer})
		layer = nil
	}
	return
}

func (p *ReusingLayerParser) recycle(layers []gopacket.DecodingLayer) {
	for _, layer := range layers {
		typ := layer.CanDecode().LayerTypes()[0]
		p.layerCache[typ] = append(p.layerCache[typ], layer)
	}
}

func (p *ReusingLayerParser) getDecodingLayer(typ gopacket.LayerType) (layer gopacket.DecodingLayer, err error) {
	if layers, ok := p.layerCache[typ]; ok && len(layers) > 0 {
		layer = layers[len(layers)-1]
		layers = layers[:len(layers)-1]
		p.layerCache[typ] = layers
		return
	}
	if factory, ok := p.factories[typ]; ok {
		layer = factory.Create(typ)
	} else {
		err = gopacket.UnsupportedLayerType(typ)
	}
	return
}

func (p *ReusingLayerParser) SetTruncated() {
	p.Truncated = true
}

type SingleDecodingLayerFactory struct {
	layerType gopacket.LayerType
	create    func() gopacket.DecodingLayer
}

var _ DecodingLayerFactory = &SingleDecodingLayerFactory{}

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

type UnknownDecodingLayer struct {
	Data []byte
}

var (
	_ gopacket.Layer         = &UnknownDecodingLayer{}
	_ gopacket.DecodingLayer = &UnknownDecodingLayer{}
)

func (d *UnknownDecodingLayer) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	d.Data = data
	return nil
}
func (d *UnknownDecodingLayer) CanDecode() gopacket.LayerClass    { return d.LayerType() }
func (d *UnknownDecodingLayer) NextLayerType() gopacket.LayerType { return gopacket.LayerTypeZero }
func (d *UnknownDecodingLayer) LayerType() gopacket.LayerType     { return gopacket.LayerTypeDecodeFailure }
func (d *UnknownDecodingLayer) LayerContents() []byte             { return d.Data }
func (d *UnknownDecodingLayer) LayerPayload() []byte              { return nil }

var UnknownDecodingLayerFactory = NewSingleDecodingLayerFactory(
	gopacket.LayerTypeDecodeFailure,
	func() gopacket.DecodingLayer { return &UnknownDecodingLayer{} },
)

var _ = log.Ldate
