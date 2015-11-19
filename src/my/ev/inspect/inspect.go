// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package inspect

import (
	"bytes"
	"fmt"

	"github.com/go-yaml/yaml"
	"github.com/ikravets/errs"

	"my/ev/inspect/device"
)

// XXX field tags are only relevant here for marshalling
type block struct {
	Name string `yaml:",omitempty"`
	Desc string `yaml:",omitempty"`
	Regs []*register
}
type register struct {
	Addr   uint64
	Name   string   `yaml:",omitempty"`
	Desc   string   `yaml:",omitempty"`
	Good   *uint64  `yaml:",omitempty"`
	Fields []*field `yaml:",omitempty"`
	value  uint64
	isBad  bool
}
type field struct {
	Bits  []uint  `yaml:",flow,omitempty"`
	Width uint    `yaml:",omitempty"`
	Name  string  `yaml:",omitempty"`
	Desc  string  `yaml:",omitempty"`
	Good  *uint64 `yaml:",omitempty"`
	value uint64
	isBad bool
	reg   *register
}

type Config struct {
	ast []*block
}

func NewConfig() *Config {
	return &Config{}
}
func (c *Config) Parse(yamlDoc string) (err error) {
	defer errs.PassE(&err)
	errs.CheckE(yaml.Unmarshal([]byte(yamlDoc), &c.ast))
	for _, block := range c.ast {
		for _, register := range block.Regs {
			var currentLSB uint = 0
			for _, field := range register.Fields {
				check := func(cond bool, s string) {
					if !cond {
						errs.CheckE(fmt.Errorf("`%s`/`%s`/`%s`: %s", block.Name, register.Name, field.Name, s))
					}
				}

				// the following ensures that field.Width == field.Bits[1] - field.Bits[0] + 1
				l := len(field.Bits)
				if l == 1 {
					field.Bits = append(field.Bits, field.Bits[0])
				}
				check(l <= 2, "too many bits specified")
				if l == 0 {
					check(field.Width != 0, "missing field location")
					field.Bits = []uint{currentLSB, currentLSB + field.Width - 1}
				} else {
					if field.Bits[0] > field.Bits[1] {
						field.Bits = []uint{field.Bits[1], field.Bits[0]}
					}
					width := field.Bits[1] - field.Bits[0] + 1
					if field.Width == 0 {
						field.Width = width
					}
					check(field.Width == width, "field width inconsistent")
				}

				check(currentLSB <= field.Bits[0], "field is out of order")
				check(63 >= field.Bits[1], "field out of range")
				currentLSB = field.Bits[1] + 1
			}
		}
	}
	return
}

func (c *Config) Dump() (yamlDoc string, err error) {
	defer errs.PassE(&err)
	buf, err := yaml.Marshal(c.ast)
	errs.CheckE(err)
	yamlDoc = string(buf)
	return
}

var blockNameFormat = `

---------------------------------
**      %s     **
---------------------------------
`

func (c *Config) Report() string {
	var buf bytes.Buffer
	for _, block := range c.ast {
		fmt.Fprintf(&buf, blockNameFormat, block.Name)
		fmt.Fprintf(&buf, "%s\n", block.Desc)
		for _, register := range block.Regs {
			fmt.Fprintf(&buf, "\n%s %0#16x value: %0#16x", register.Name, register.Addr, register.value)
			if register.isBad {
				fmt.Fprintf(&buf, " EXPECTED: %0#16x", *register.Good)
			}
			fmt.Fprintf(&buf, "     %s\n", register.Desc)
			for _, field := range register.Fields {
				width := int(field.Width+3) / 4
				fmt.Fprintf(&buf, "%s: %0#[2]*x %[3]d", field.Name, width, field.value)
				if field.isBad {
					fmt.Fprintf(&buf, " EXPECTED: %0#[1]*x %[2]d", width, *field.Good)
				}
				fmt.Fprintf(&buf, "     %s\n", field.Desc)
			}
		}
	}
	return buf.String()
}

func (c *Config) Probe(dev device.Device) (err error) {
	defer errs.PassE(&err)
	for _, block := range c.ast {
		for _, register := range block.Regs {
			register.value, err = dev.ReadRegister(4, register.Addr, 8)
			errs.CheckE(err)
			register.isBad = register.Good != nil && register.value != *register.Good
			for _, field := range register.Fields {
				field.value = register.value >> field.Bits[0] & (1<<field.Width - 1)
				field.isBad = field.Good != nil && field.value != *field.Good
			}
		}
	}
	return
}
