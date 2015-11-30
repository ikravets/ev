// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package inspect

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"strings"

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

	GoodExpr string `yaml:",omitempty"`
}
type field struct {
	Bits  []uint  `yaml:",flow,omitempty"`
	Width uint    `yaml:",omitempty"`
	Name  string  `yaml:",omitempty"`
	Desc  string  `yaml:",omitempty"`
	Good  *uint64 `yaml:",omitempty"`
	value uint64
	isBad bool

	GoodExpr string `yaml:",omitempty"`
}

type Config struct {
	ast   []*block
	isBad bool
	dev   device.Device

	values map[string]uint64
}

func NewConfig(dev device.Device) *Config {
	return &Config{
		dev: dev,

		values: make(map[string]uint64),
	}
}

// check only valid after Probe
func (c *Config) IsBad() bool {
	return c.isBad
}

func checkC(cond bool, errloc, s string) {
	if !cond {
		errs.CheckE(fmt.Errorf("%s: %s", errloc, s))
	}
}
func (c *Config) processGood(name string, expr *string, good *uint64, errloc string) {
	c.values[name] = 0
	if strings.Index(*expr, "%s") != -1 {
		*expr = fmt.Sprintf(*expr, name)
	}
	checkC(good == nil || *expr == "", errloc, "both Good and GoodExpr specified")
}
func (c *Config) Parse(yamlDoc string) (err error) {
	defer errs.PassE(&err)
	errs.CheckE(yaml.Unmarshal([]byte(yamlDoc), &c.ast))
	for _, block := range c.ast {
		for _, reg := range block.Regs {
			var currentLSB uint = 0
			c.processGood(reg.Name, &reg.GoodExpr, reg.Good, fmt.Sprintf("`%s`/`%s`", block.Name, reg.Name))
			for _, f := range reg.Fields {
				loc := fmt.Sprintf("`%s`/`%s`/`%s`", block.Name, reg.Name, f.Name)
				c.processGood(f.Name, &f.GoodExpr, f.Good, loc)

				// the following ensures that f.Width == f.Bits[1] - f.Bits[0] + 1
				l := len(f.Bits)
				if l == 1 {
					f.Bits = append(f.Bits, f.Bits[0])
				}
				checkC(l <= 2, loc, "too many bits specified")
				if l == 0 {
					checkC(f.Width != 0, loc, "missing field location")
					f.Bits = []uint{currentLSB, currentLSB + f.Width - 1}
				} else {
					if f.Bits[0] > f.Bits[1] {
						f.Bits = []uint{f.Bits[1], f.Bits[0]}
					}
					width := f.Bits[1] - f.Bits[0] + 1
					if f.Width == 0 {
						f.Width = width
					}
					checkC(f.Width == width, loc, "field width inconsistent")
				}

				checkC(currentLSB <= f.Bits[0], loc, "field is out of order")
				checkC(63 >= f.Bits[1], loc, "field out of range")
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
				fmt.Fprintf(&buf, " EXPECTED: %s ", reg.GoodExpr)
				if reg.Good != nil {
					fmt.Fprintf(&buf, "%0#16x", *reg.Good)
				}
			}
			fmt.Fprintf(&buf, "     %s\n", reg.Desc)
			for _, f := range reg.Fields {
				width := int(f.Width+3) / 4
				fmt.Fprintf(&buf, "%s: %0#*x %[3]d", f.Name, width, f.value)
				if f.isBad {
					fmt.Fprintf(&buf, " EXPECTED: %s ", f.GoodExpr)
					if f.Good != nil {
						fmt.Fprintf(&buf, "%0#*x %[2]d", width, *f.Good)
					}
				}
				fmt.Fprintf(&buf, "     %s\n", f.Desc)
			}
		}
	}
	return buf.String()
}

func (c *Config) checkValue(name, good string, value uint64) bool {
	buf := bytes.NewBufferString("package config\nconst (\n")
	length := 0
	for name, value := range c.values {
		fmt.Fprintf(buf, "\t%s = %d\n", name, value)
		if len(name) > length {
			length = len(name)
		}
	}
	cname := strings.Repeat("_", length+1)
	fmt.Fprintf(buf, "\t%s = %s\n)\n", cname, good)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "config", buf.String(), 0)
	errs.CheckE(err)
	var conf types.Config
	p, err := conf.Check(f.Name.Name, fset, []*ast.File{f}, &types.Info{})
	errs.CheckE(err)
	return constant.BoolVal(p.Scope().Lookup(cname).(*types.Const).Val())
}
func (c *Config) Probe() (err error) {
	defer errs.PassE(&err)
	c.isBad = false
	for _, block := range c.ast {
		for _, reg := range block.Regs {
			reg.value, err = c.dev.ReadRegister(4, reg.Addr, 8)
			errs.CheckE(err)
			c.values[reg.Name] = reg.value
			reg.isBad = reg.Good != nil && reg.value != *reg.Good || reg.GoodExpr != "" && !c.checkValue(reg.Name, reg.GoodExpr, reg.value)
			c.isBad = c.isBad || reg.isBad
			for _, f := range reg.Fields {
				f.value = reg.value >> f.Bits[0] & (1<<f.Width - 1)
				c.values[f.Name] = f.value
				f.isBad = f.Good != nil && f.value != *f.Good || f.GoodExpr != "" && !c.checkValue(f.Name, f.GoodExpr, f.value)
				c.isBad = c.isBad || f.isBad
			}
		}
	}
	return
}
