// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"log"

	"github.com/jessevdk/go-flags"
)

type cmdLegacy struct {
	commands []Extender
}

func (c *cmdLegacy) Execute(args []string) error {
	return nil
}

func (c *cmdLegacy) ConfigParser(parser *flags.Parser) {
	legacy, err := parser.AddCommand("legacy", "run legacy commands", "", c)
	if err != nil {
		log.Fatalln(err)
	}
	c.commands = append(c.commands, &cmdOldPcap2log{}, &cmdOldPcap2memh{})
	for _, lcmd := range c.commands {
		dummyParser := flags.NewParser(nil, flags.None)
		lcmd.ConfigParser(dummyParser)
		cmd := dummyParser.Commands()[0]
		legacy.AddCommand(cmd.Name, cmd.ShortDescription, cmd.LongDescription, lcmd)
	}
}

func (c *cmdLegacy) ParsingFinished() {
	for _, cmd := range c.commands {
		cmd.ParsingFinished()
	}
}

func init() {
	var c cmdLegacy
	Registry.Register(&c)
}
