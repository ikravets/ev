// Copyright (c) Ilia Kravets, 2016. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sim

import "my/ev/packet"

type Options interface {
	PriceScale(oid packet.OptionId) int
	SetPriceScale(oid packet.OptionId, priceScale int)
	ApplyOperation(operation SimOperation)
}

type options struct {
	m map[packet.OptionId]option
	d option
}

type option struct {
	priceScale int
}

func NewOptions() Options {
	return &options{
		m: make(map[packet.OptionId]option),
		d: option{priceScale: packet.PriceDefaultDec},
	}
}
func (o *options) PriceScale(oid packet.OptionId) int {
	if v, ok := o.m[oid]; ok {
		return v.priceScale
	}
	return o.d.priceScale
}
func (o *options) SetPriceScale(oid packet.OptionId, priceScale int) {
	v := o.m[oid]
	v.priceScale = priceScale
	o.m[oid] = v
}
func (o *options) ApplyOperation(operation SimOperation) {
	_ = operation.(*OperationScale)
	o.SetPriceScale(operation.GetOptionId(), operation.GetPrice())
}
