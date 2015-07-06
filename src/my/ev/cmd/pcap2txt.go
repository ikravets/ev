// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/google/gopacket/pcap"
	"github.com/jessevdk/go-flags"

	"my/errs"

	"my/ev/packet"
	"my/ev/packet/processor"
)

type cmdPcap2txt struct {
	InputFileName  string `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	OutputFileName string `long:"output" short:"o" value-name:"FILE" default:"/dev/stdout" default-mask:"stdout" description:"output file"`
	shouldExecute  bool
}

func (c *cmdPcap2txt) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdPcap2txt) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("pcap2txt", "convert pcap file to human-readable text", "", c)
}

func (c *cmdPcap2txt) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	handle, err := pcap.OpenOffline(c.InputFileName)
	errs.CheckE(err)
	defer handle.Close()
	outFile, err := os.OpenFile(c.OutputFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	errs.CheckE(err)
	defer func() { errs.CheckE(outFile.Close()) }()

	printer := &packetPrinter{w: outFile}
	pp := processor.NewProcessor()
	pp.SetObtainer(handle)
	pp.SetHandler(printer)
	pp.ProcessAll()
}

func init() {
	var c cmdPcap2txt
	Registry.Register(&c)
}

type packetPrinter struct {
	w            io.Writer
	packetNumber int
}

func (p *packetPrinter) HandlePacket(packet packet.Packet) {
	p.packetNumber++
	_, err := fmt.Fprintf(p.w, "%d %s\n", p.packetNumber, packet)
	errs.CheckE(err)
}
func (_ *packetPrinter) HandleMessage(_ packet.ApplicationMessage) {
}
