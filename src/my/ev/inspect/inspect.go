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
		for _, reg := range block.Regs {
			var currentLSB uint = 0
			for _, f := range reg.Fields {
				check := func(cond bool, s string) {
					if !cond {
						errs.CheckE(fmt.Errorf("`%s`/`%s`/`%s`: %s", block.Name, reg.Name, f.Name, s))
					}
				}

				// the following ensures that f.Width == f.Bits[1] - f.Bits[0] + 1
				l := len(f.Bits)
				if l == 1 {
					f.Bits = append(f.Bits, f.Bits[0])
				}
				check(l <= 2, "too many bits specified")
				if l == 0 {
					check(f.Width != 0, "missing field location")
					f.Bits = []uint{currentLSB, currentLSB + f.Width - 1}
				} else {
					if f.Bits[0] > f.Bits[1] {
						f.Bits = []uint{f.Bits[1], f.Bits[0]}
					}
					width := f.Bits[1] - f.Bits[0] + 1
					if f.Width == 0 {
						f.Width = width
					}
					check(f.Width == width, "field width inconsistent")
				}

				check(currentLSB <= f.Bits[0], "field is out of order")
				check(63 >= f.Bits[1], "field out of range")
				currentLSB = f.Bits[1] + 1
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

func (c *Config) Report() string {
	var buf bytes.Buffer
	pref := map[bool]byte{false: ' ', true: '*'}
	nd := func(name, desc string) string { return fmt.Sprintf("%-55.55s %s", name, desc) }
	for _, block := range c.ast {
		fmt.Fprintf(&buf, "  %s\n", nd(block.Name, block.Desc))
		for _, reg := range block.Regs {
			fmt.Fprintf(&buf, "%c%12c%0#16x %0#16x %s\n", pref[reg.isBad], ' ', reg.value, reg.Addr, nd(reg.Name, reg.Desc))
			for _, f := range reg.Fields {
				mid := fmt.Sprintf("-%-8d", f.Bits[1])
				if f.Width == 1 {
					mid = fmt.Sprintf("%9c", ' ')
				}
				fmt.Fprintf(&buf, "%c %-10d %0#-18.*[2]x %9[4]d%s %s\n", pref[f.isBad], f.value, int(f.Width+3)/4, f.Bits[0], mid, nd(f.Name, f.Desc))
			}
		}
	}
	return buf.String()
}

func (c *Config) ReportLegacy() string {
	var buf bytes.Buffer
	for _, block := range c.ast {
		fmt.Fprintf(&buf, "\n**      %s      **\n", block.Name)
		fmt.Fprintf(&buf, "%s\n", block.Desc)
		for _, reg := range block.Regs {
			fmt.Fprintf(&buf, "\n%s %0#16x value: %0#16x", reg.Name, reg.Addr, reg.value)
			if reg.isBad {
				fmt.Fprintf(&buf, " EXPECTED: %0#16x", *reg.Good)
			}
			fmt.Fprintf(&buf, "     %s\n", reg.Desc)
			for _, f := range reg.Fields {
				width := int(f.Width+3) / 4
				fmt.Fprintf(&buf, "%s: %0#*x %[3]d", f.Name, width, f.value)
				if f.isBad {
					fmt.Fprintf(&buf, " EXPECTED: %0#*x %[2]d", width, *f.Good)
				}
				fmt.Fprintf(&buf, "     %s\n", f.Desc)
			}
		}
	}
	return buf.String()
}

func (c *Config) Probe(dev device.Device) (err error) {
	defer errs.PassE(&err)
	for _, block := range c.ast {
		for _, reg := range block.Regs {
			reg.value, err = dev.ReadRegister(4, reg.Addr, 8)
			errs.CheckE(err)
			reg.isBad = reg.Good != nil && reg.value != *reg.Good
			for _, f := range reg.Fields {
				f.value = reg.value >> f.Bits[0] & (1<<f.Width - 1)
				f.isBad = f.Good != nil && f.value != *f.Good
			}
		}
	}
	return
}
