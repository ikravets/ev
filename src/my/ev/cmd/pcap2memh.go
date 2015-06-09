// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"log"

	"github.com/google/gopacket/pcap"
	"github.com/jessevdk/go-flags"

	"my/ev/packet/processor"
	"my/ev/rec"
)

type cmdPcap2memh struct {
	InputFileName  string `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	DestDirName    string `short:"d" long:"dest-dir" default:"." default-mask:"current dir" value-name:"DIR" description:"destination directory, will be created if does not exist" `
	PacketNumLimit int    `long:"count" short:"c" value-name:"NUM" description:"limit number of input packets"`
	shouldExecute  bool
}

func (c *cmdPcap2memh) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdPcap2memh) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("pcap2memh", "convert pcap file to readmemh simulator input", "", c)
}

func (c *cmdPcap2memh) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	handle, err := pcap.OpenOffline(c.InputFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	printer, err := rec.NewMemhRecorder(c.DestDirName)
	if err != nil {
		log.Fatal(err)
	}
	defer printer.Close()
	printer.AddDummy()

	pp := processor.NewProcessor()
	pp.LimitPacketNumber(c.PacketNumLimit)
	pp.SetObtainer(handle)
	pp.SetHandler(printer)
	pp.ProcessAll()
}

func init() {
	var c cmdPcap2memh
	Registry.Register(&c)
}
