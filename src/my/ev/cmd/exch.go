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
	Raddr         string `long:"remote-addr"value-name:"IPADDR" default:"224.0.131.2:30110" description:"remote server address"`
	Interactive   bool   `long:"interactive" short:"i" description:"run interactively"`
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
		Protocol:    c.Type,
		LocalAddr:   c.Laddr,
		RemoteAddr:  c.Raddr,
		Interactive: c.Interactive,
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
