// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package inspect

import (
	"gopkg.in/yaml.v2"

	"my/ev/inspect/device"
)

type Node interface {
	Name() string
	Desc() string
	Addr() string
	Value() (uint64, error)
	IsBad() bool
	Children() []Node
}

// XXX field tags are only relevant here for marshalling
type block struct {
	Name string `yaml:",omitempty"`
	Desc string `yaml:",omitempty"`
	Regs []register
}
type register struct {
	Addr   uint64
	Name   string  `yaml:",omitempty"`
	Desc   string  `yaml:",omitempty"`
	Good   *uint64 `yaml:",omitempty"`
	Fields []field `yaml:",omitempty"`
	value  uint64
}
type field struct {
	Bits  []uint  `yaml:",flow,omitempty"`
	Width uint    `yaml:",omitempty"`
	Name  string  `yaml:",omitempty"`
	Desc  string  `yaml:",omitempty"`
	Good  *uint64 `yaml:",omitempty"`
	value uint64
	reg   *register
}

// FIXME uncomment and implement
//var _ Node = &Block{}
//var _ Node = &Register{}
//var _ Node = &Field{}

func ParseConf(yamlDoc string) (root Node, err error) {
	var b block
	err = yaml.Unmarshal([]byte(yamlDoc), &b)
	// FIXME
	// root = &b
	return
}

func DumpConf(root Node) (yamlDoc string, err error) {
	// TODO
	return
}

func Report(root Node) (report string, err error) {
	// TODO
	return
}

func Probe(dev device.Device, root Node) (err error) {
	// TODO
	return
}
