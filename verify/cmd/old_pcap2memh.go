// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"log"
	"os"

	"github.com/jessevdk/go-flags"

	"my/itto/verify/legacy/packet"
	"my/itto/verify/legacy/pcap2memh"
)

type cmdOldPcap2memh struct {
	DestDirName   string                        `short:"d" long:"dest-dir" default:"." default-mask:"current dir" value-name:"DIR" description:"destination directory, will be created if does not exist" `
	InputFileName string                        `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	Args          struct{ TsharkArgs []string } `positional-args:"y"`
	shouldExecute bool
}

func (c *cmdOldPcap2memh) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdOldPcap2memh) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("pcap2memh", "convert pcap file to simulator input", "", c)
}

func (c *cmdOldPcap2memh) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	args := append([]string{"-x"}, c.Args.TsharkArgs...)
	dumpReader, finisher := packet.TsharkOpen(c.InputFileName, args)
	defer finisher()
	if err := os.MkdirAll(c.DestDirName, 0755); err != nil {
		log.Fatal(err)
	}
	pcap2memh.Translate(dumpReader, c.DestDirName)
}

/* legacy command, see legacy.go
func init() {
	var c cmdOldPcap2memh
	Registry.Register(&c)
}*/
