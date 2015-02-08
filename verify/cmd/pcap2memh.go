// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/pcap"
	"github.com/jessevdk/go-flags"

	"my/itto/verify/packet"
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

	printer, err := newMemhPrinter(c.DestDirName)
	if err != nil {
		log.Fatal(err)
	}
	defer printer.Close()

	pp := packet.NewProcessor()
	pp.LimitPacketNumber(c.PacketNumLimit)
	pp.SetObtainer(handle)
	pp.SetHandler(printer)
	pp.ProcessAll()
}

func init() {
	var c cmdPcap2memh
	Registry.Register(&c)
}

type memhPrinter struct {
	dir           string
	packetNum     int
	indexFile     io.WriteCloser
	packetLengths []int
	hexDigits     []byte
	outbuf        *bytes.Buffer
}

func newMemhPrinter(dir string) (p *memhPrinter, err error) {
	p = &memhPrinter{dir: dir}
	p.hexDigits = []byte("0123456789abcdef")
	p.outbuf = bytes.NewBuffer(make([]byte, 0, 4096))
	if err = os.MkdirAll(p.dir, 0755); err != nil {
		return
	}
	indexFileName := filepath.Join(p.dir, "packet.length")
	if p.indexFile, err = os.OpenFile(indexFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644); err != nil {
		log.Fatal(err)
		return
	}
	return
}

func (p *memhPrinter) Close() {
	fmt.Fprintf(p.indexFile, "%x\n", p.packetNum)
	for _, l := range p.packetLengths {
		fmt.Fprintf(p.indexFile, "%x\n", l)
	}
	p.indexFile.Close()
}

func (p *memhPrinter) HandlePacket(packet gopacket.Packet) {
	p.outbuf.Reset()
	d := packet.Data()
	chunk := make([]byte, 8)
	zeroChunk := make([]byte, 8)
	for i := 0; i < len(d); i += len(chunk) {
		if len(d)-i < len(chunk) {
			copy(chunk, zeroChunk)
		}
		copy(chunk, d[i:])
		for i := len(chunk) - 1; i >= 0; i-- {
			b := chunk[i]
			p.outbuf.WriteByte(p.hexDigits[b/16])
			p.outbuf.WriteByte(p.hexDigits[b%16])
		}
		p.outbuf.WriteByte('\n')
	}

	packetLength := (len(d) + len(chunk) - 1) / len(chunk)
	p.packetLengths = append(p.packetLengths, packetLength)
	dataFileName := filepath.Join(p.dir, fmt.Sprintf("packet.readmemh%d", p.packetNum))
	if err := ioutil.WriteFile(dataFileName, p.outbuf.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
	p.packetNum++
}

func (_ *memhPrinter) HandleMessage(_ packet.ApplicationMessage) {}
