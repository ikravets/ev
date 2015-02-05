// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"log"

	"github.com/jessevdk/go-flags"

	"my/itto/verify/efhsim"
)

type cmdEfhsim struct {
	InputFileName     string `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	OutputFileNameSim string `long:"output-sim" short:"s" value-name:"FILE" description:"output file for hw simulator"`
	PacketNumLimit    int    `long:"count" short:"c" value-name:"NUM" description:"limit number of input packets"`
	shouldExecute     bool
}

func (c *cmdEfhsim) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdEfhsim) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("efhsim",
		"simulate efh",
		"",
		c)
}

func (c *cmdEfhsim) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	efh := efhsim.NewEfhSim()
	efh.SetInput(c.InputFileName, c.PacketNumLimit)
	if c.OutputFileNameSim != "" {
		if err := efh.SetOutputSimLog(c.OutputFileNameSim); err != nil {
			log.Fatal(err)
		}
	}
	if err := efh.AnalyzeInput(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	var c cmdEfhsim
	Registry.Register(&c)
}
