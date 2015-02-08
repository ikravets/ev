// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"log"
	"os"

	"github.com/jessevdk/go-flags"

	"my/itto/verify/packet"
	"my/itto/verify/pcap2memh"
)

type cmdPcap2memh struct {
	DestDirName   string                        `short:"d" long:"dest-dir" default:"." default-mask:"current dir" value-name:"DIR" description:"destination directory, will be created if does not exist" `
	InputFileName string                        `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	Args          struct{ TsharkArgs []string } `positional-args:"y"`
	shouldExecute bool
}

func (p *cmdPcap2memh) Execute(args []string) error {
	p.shouldExecute = true
	return nil
}

func (p *cmdPcap2memh) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("pcap2memh",
		"convert pcap file to simulator input",
		"",
		p)
}

func (p *cmdPcap2memh) ParsingFinished() {
	if !p.shouldExecute {
		return
	}
	args := append([]string{"-x"}, p.Args.TsharkArgs...)
	dumpReader, finisher := packet.TsharkOpen(p.InputFileName, args)
	defer finisher()
	if err := os.MkdirAll(p.DestDirName, 0755); err != nil {
		log.Fatal(err)
	}
	pcap2memh.Translate(dumpReader, p.DestDirName)
}

func init() {
	var c cmdPcap2memh
	Registry.Register(&c)
}
