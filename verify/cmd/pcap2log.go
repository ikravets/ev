// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"log"
	"os"

	"github.com/jessevdk/go-flags"

	"my/itto/verify/packet"
	"my/itto/verify/pcap2log"
)

type cmdPcap2log struct {
	InputFileName  string                        `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	OutputFileName string                        `long:"output" short:"o" value-name:"FILE" default:"/dev/stdout" default-mask:"stdout" description:"output file"`
	Args           struct{ TsharkArgs []string } `positional-args:"y"`
	shouldExecute  bool
}

func (p *cmdPcap2log) Execute(args []string) error {
	p.shouldExecute = true
	return nil
}

func (p *cmdPcap2log) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("pcap2log",
		"convert pcap file to simulator output",
		"",
		p)
}

func (p *cmdPcap2log) ParsingFinished() {
	if !p.shouldExecute {
		return
	}
	args := []string{
		"-d", "udp.port==18000:10,moldudp64",
		"-V",
	}
	args = append(args, p.Args.TsharkArgs...)
	dumpReader, finisher := packet.TsharkOpen(p.InputFileName, args)
	defer finisher()
	outFile, err := os.OpenFile(p.OutputFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()
	t := pcap2log.NewTranslator(dumpReader, outFile)
	t.Translate()
}

func init() {
	var c cmdPcap2log
	Registry.Register(&c)
}
