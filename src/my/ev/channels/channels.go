// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package channels

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
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

func ParseChannel(channel string) (ch *net.UDPAddr, subch int, err error) {
	defer errs.PassE(&err)
	subch = -1
	if fields := strings.Split(channel, ":"); len(fields) == 3 {
		ch, err = net.ResolveUDPAddr("udp", strings.Join(fields[:2], ":"))
		errs.CheckE(err)
		subch, err = strconv.Atoi(fields[2])
	} else {
		ch, err = net.ResolveUDPAddr("udp", channel)
		errs.CheckE(err)
	}
	return
}

func (c *config) LoadFromStr(nameOrAddr string) (err error) {
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
		_, _, err = ParseChannel(nameOrAddr)
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

func IPFlow(addr *net.UDPAddr) gopacket.Flow {
	return gopacket.NewFlow(layers.EndpointIPv4, nil, addr.IP.To4())
}

func UDPFlow(addr *net.UDPAddr) gopacket.Flow {
	portBytes := []byte{byte(addr.Port >> 8), byte(addr.Port)}
	return gopacket.NewFlow(layers.EndpointUDPPort, nil, portBytes)
}

var EndpointSubchannelMetadata = gopacket.EndpointTypeMetadata{"subchannel", func(b []byte) string {
	return strconv.Itoa(int(binary.LittleEndian.Uint32(b)))
}}
var EndpointSubchannel = gopacket.RegisterEndpointType(13000, EndpointSubchannelMetadata)

func SubchannelFlow(s int) gopacket.Flow {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(s))
	return gopacket.NewFlow(EndpointSubchannel, nil, b)
}
