// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"github.com/ikravets/errs"
	"github.com/jessevdk/go-flags"

	"my/ev/exch"
)

type cmdExch struct {
	Type          string `long:"type" short:"t" value-name:"EXCH" default:"nasdaq" description:"exchange type: nasdaq, bats"`
	Laddr         string `long:"local-addr" value-name:"IPADDR" default:"10.2.0.5:0" description:"local address"`
	RTMCaddr      string `long:"feed-multicast" value-name:"IPADDR" default:"224.0.131.0:30101" description:"feed server mcast address"`
	GRMCaddr      string `long:"gap-multicast" value-name:"IPADDR" default:"233.130.124.0:30101" description:"gap server mcast address"`
	ConnNumLimit  int    `long:"count" short:"c" value-name:"NUM" default:"1" description:"limit number of connections"`
	Interactive   bool   `long:"interactive" short:"i" description:"run interactively"`
	GapPeriod     uint64 `long:"gap-period" short:"g" value-name:"NUM" default:"0xFFFFFFFFFFFFFF" description:"period simulate gap"`
	GapSize       uint64 `long:"gap-limit" short:"s" value-name:"NUM" default:"0" description:"limit number of gap messages"`
	shouldExecute bool
}

func (c *cmdExch) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdExch) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("exch", "run exchange simulating server", "", c)
}

func (c *cmdExch) ParsingFinished() (err error) {
	if !c.shouldExecute {
		return
	}
	conf := exch.Config{
		Protocol:     c.Type,
		LocalAddr:    c.Laddr,
		FeedAddr:     c.RTMCaddr,
		GapAddr:      c.GRMCaddr,
		Interactive:  c.Interactive,
		GapPeriod:    c.GapPeriod,
		GapSize:      c.GapSize,
		ConnNumLimit: c.ConnNumLimit,
	}
	es, err := exch.NewExchangeSimulator(conf)
	errs.CheckE(err)
	es.Run()
	return
}

func init() {
	var c cmdExch
	Registry.Register(&c)
}
