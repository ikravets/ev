// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"github.com/ikravets/errs"
	"github.com/jessevdk/go-flags"

	"my/ev/packet"
)

type cmdReplay struct {
	InputFileName   string `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	OutputInterface string `long:"iface" required:"y" value-name:"IFACE" description:"output interface name"`
	Pps             int    `long:"pps"   short:"p" value-name:"NUM" description:"packets per second"`
	Limit           int    `long:"limit" short:"L" value-name:"NUM" description:"stop after NUM packets"`
	Loop            int    `long:"loop"  short:"l" value-name:"NUM" description:"loop NUM times"`
	shouldExecute   bool
}

func (c *cmdReplay) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdReplay) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("replay", "replay pcap dump", "", c)
}

func (c *cmdReplay) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	r := packet.Replay{
		IfaceName: c.OutputInterface,
		DumpName:  c.InputFileName,
		Limit:     c.Limit,
		Pps:       c.Pps,
	}
	loop := c.Loop
	if loop == 0 {
		loop = 1
	}
	for i := 0; i < loop; i++ {
		errs.CheckE(r.Run())
	}
}

func init() {
	var c cmdReplay
	Registry.Register(&c)
}
