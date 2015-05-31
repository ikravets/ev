// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"log"

	"code.google.com/p/gopacket"

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
		layer, err := p.getDecodingLayer(tp.Type)
		errs.CheckE(err)
		//errs.CheckE(layer.DecodeFromBytes(tp.Payload, p.df))
		layer.DecodeFromBytes(tp.Payload, p.df) // FIXME check returned error
		*decoded = append(*decoded, layer)

		if dml, ok := layer.(DecodingMultiLayer); ok {
			tps = append(tps, dml.NextLayers()...)
		} else if len(layer.LayerPayload()) > 0 {
			tps = append(tps, TypedPayload{Type: layer.NextLayerType(), Payload: layer.LayerPayload()})
		}
	}
	return nil
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

var _ = log.Ldate
