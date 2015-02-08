// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"log"
	"os"

	"github.com/jessevdk/go-flags"

	"my/itto/verify/legacy/packet"
	"my/itto/verify/legacy/pcap2log"
)

type cmdOldPcap2log struct {
	InputFileName  string                        `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	OutputFileName string                        `long:"output" short:"o" value-name:"FILE" default:"/dev/stdout" default-mask:"stdout" description:"output file"`
	Args           struct{ TsharkArgs []string } `positional-args:"y"`
	shouldExecute  bool
}

func (c *cmdOldPcap2log) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdOldPcap2log) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("pcap2log", "convert pcap file to simulator output", "", c)
}

func (c *cmdOldPcap2log) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	args := []string{
		"-d", "udp.port==18000:10,moldudp64",
		"-V",
	}
	args = append(args, c.Args.TsharkArgs...)
	dumpReader, finisher := packet.TsharkOpen(c.InputFileName, args)
	defer finisher()
	outFile, err := os.OpenFile(c.OutputFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()
	t := pcap2log.NewTranslator(dumpReader, outFile)
	t.Translate()
}

/* legacy command, see legacy.go
func init() {
	var c cmdOldPcap2log
	Registry.Register(&c)
}*/
