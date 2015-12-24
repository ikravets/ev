// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package channels

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strings"

	"github.com/ikravets/errs"
)

type Config interface {
	LoadFromReader(rd io.Reader) error
	LoadFromStr(nameOrAddr string) error
	Addrs() []string
}

type config struct {
	addrs []string
}

var _ Config = &config{}

func NewConfig() Config {
	return &config{}
}

func (c *config) LoadFromReader(rd io.Reader) (err error) {
	defer errs.PassE(&err)
	all, err := ioutil.ReadAll(rd)
	errs.CheckE(err)
	for _, str := range strings.Fields(string(all)) {
		errs.CheckE(c.LoadFromStr(str))
	}
	return
}

func (c *config) LoadFromStr(nameOrAddr string) (err error) {
	defer errs.PassE(&err)
	switch nameOrAddr {
	case "nasdaq":
		for i := 0; i < 4; i++ {
			c.addAddr(fmt.Sprintf("233.54.12.%d:%d", 1+i, 18001+i))
		}
	case "bats":
		for i := 0; i < 32; i++ {
			c.addAddr(fmt.Sprintf("224.0.131.%d:%d", i/4, 30101+i))
		}
	case "bats-b":
		for i := 0; i < 32; i++ {
			c.addAddr(fmt.Sprintf("233.130.124.%d:%d", i/4, 30101+i))
		}
	case "miax":
		for i := 0; i < 24; i++ {
			c.addAddr(fmt.Sprintf("224.0.105.%d:%d", 1+i, 51001+i))
		}
	default:
		_, err = net.ResolveUDPAddr("udp", nameOrAddr)
		errs.CheckE(err)
		c.addAddr(nameOrAddr)
	}
	return
}

func (c *config) Addrs() []string {
	return c.addrs
}

func (c *config) addAddr(addr string) {
	c.addrs = append(c.addrs, addr)
	return
}
