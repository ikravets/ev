// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"github.com/jessevdk/go-flags"

	"my/ev/echo"
)

type cmdEcho struct {
	shouldExecute bool
}

func (c *cmdEcho) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdEcho) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("echo", "run echo server", "", c)
}

func (c *cmdEcho) ParsingFinished() (err error) {
	if !c.shouldExecute {
		return
	}
	es := echo.NewEchoServer()
	es.Run()
	return
}

func init() {
	var c cmdEcho
	Registry.Register(&c)
}
