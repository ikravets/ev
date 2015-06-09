// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"github.com/jessevdk/go-flags"

	"my/itto/verify/exch"
)

type cmdExch struct {
	shouldExecute bool
}

func (c *cmdExch) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdExch) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("exch", "run exchange simulating server", "", c)
}

func (c *cmdExch) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	es := exch.NewExchangeSimulatorServer()
	es.Run()
}

func init() {
	var c cmdExch
	Registry.Register(&c)
}
