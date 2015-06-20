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
	layerCache map[gopacket.LayerType]*[]gopacket.DecodingLayer
	df         gopacket.DecodeFeedback
	Truncated  bool
	tps        []TypedPayload
}

func NewReusingLayerParser(first gopacket.LayerType, factories ...DecodingLayerFactory) *ReusingLayerParser {
	p := &ReusingLayerParser{
		factories:  make(map[gopacket.LayerType]DecodingLayerFactory),
		layerCache: make(map[gopacket.LayerType]*[]gopacket.DecodingLayer),
		first:      first,
		tps:        make([]TypedPayload, 100),
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
	defer errs.PassE(&err)
	p.Truncated = false

	dlEnd := 0
	p.tps[0] = TypedPayload{Type: p.first, Payload: data}
	for tpsStart, tpsEnd := 0, 1; tpsStart < tpsEnd; tpsStart++ {
		tp := &p.tps[tpsStart]
		if tp.Type == gopacket.LayerTypeZero {
			continue
		}
		if dlEnd == len(*decoded) {
			*decoded = append(*decoded, nil)
		}
		layer := (*decoded)[dlEnd]
		layer, err := p.tryDecode(tp, layer)
		if err != nil {
			log.Printf("error: %s\n", err)
			tp.Type = gopacket.LayerTypeDecodeFailure
			layer, err = p.tryDecode(tp, layer)
			errs.CheckE(err)
		}
		(*decoded)[dlEnd] = layer
		dlEnd++

		if dml, ok := layer.(DecodingMultiLayer); ok {
			nls := dml.NextLayers()
			p.checkTpsSize(tpsEnd, len(nls))
			copy(p.tps[tpsEnd:], nls)
			tpsEnd += len(nls)
		} else if len(layer.LayerPayload()) > 0 {
			p.checkTpsSize(tpsEnd, 1)
			p.tps[tpsEnd] = TypedPayload{Type: layer.NextLayerType(), Payload: layer.LayerPayload()}
			tpsEnd++
		}
	}
	p.recycle((*decoded)[dlEnd:])
	*decoded = (*decoded)[:dlEnd]
	return nil
}

func (p *ReusingLayerParser) checkTpsSize(tpsEnd int, add int) {
	if len(p.tps) < tpsEnd+add {
		oldTps := p.tps[:tpsEnd]
		p.tps = make([]TypedPayload, len(p.tps)*2+add)
		copy(p.tps, oldTps)
	}
}

func (p *ReusingLayerParser) tryDecode(tp *TypedPayload, oldLayer gopacket.DecodingLayer) (layer gopacket.DecodingLayer, err error) {
	if oldLayer != nil && oldLayer.CanDecode().Contains(tp.Type) {
		layer = oldLayer
	} else {
		if oldLayer != nil {
			p.recycleOne(oldLayer)
		}
		if layer, err = p.getDecodingLayer(tp.Type); err != nil {
			return
		}
	}
	if err = layer.DecodeFromBytes(tp.Payload, p.df); err != nil {
		p.recycleOne(layer)
		layer = nil
	}
	return
}

func (p *ReusingLayerParser) recycle(layers []gopacket.DecodingLayer) {
	for _, layer := range layers {
		p.recycleOne(layer)
	}
}
func (p *ReusingLayerParser) recycleOne(layer gopacket.DecodingLayer) {
	typ := layer.CanDecode().LayerTypes()[0]
	lc := p.layerCache[typ]
	if lc == nil {
		lc = &[]gopacket.DecodingLayer{}
		p.layerCache[typ] = lc
	}
	*lc = append(*lc, layer)
}
func (p *ReusingLayerParser) getDecodingLayer(typ gopacket.LayerType) (layer gopacket.DecodingLayer, err error) {
	if layers, ok := p.layerCache[typ]; ok && len(*layers) > 0 {
		layer = (*layers)[len(*layers)-1]
		*layers = (*layers)[:len(*layers)-1]
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
