// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/ikravets/errs"
	"github.com/jessevdk/go-flags"

	"my/ev/packet"
	"my/ev/pcapsplit"
)

type cmdPcapsplit struct {
	DestDirName      string     `short:"d" long:"dest-dir" default:"." default-mask:"current dir" value-name:"DIR" description:"destination directory, will be created if does not exist" `
	InputFileName    string     `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	PacketNumLimit   int        `long:"count" short:"c" value-name:"NUM" description:"limit number of input packets"`
	MinPacketsPerOid int        `long:"min-chain" short:"m" value-name:"NUM" description:"ignore options which appear in less than NUM packets"`
	UseEditcap       bool       `long:"editcap" short:"e" description:"don't write pcap files, just output editcap commands"`
	OptionIds        []optionId `long:"filter" short:"f" value-name:"OPTION_ID" description:"process OPTION_ID only"`
	TobBook          bool       `long:"tob" short:"t" description:"use 1-level-deep book (for exchange disseminating ToB only)"`
	shouldExecute    bool
}

func (p *cmdPcapsplit) Execute(args []string) error {
	p.shouldExecute = true
	return nil
}

func (p *cmdPcapsplit) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("pcapsplit",
		"split pcap file by option id",
		"",
		p)
}

func (p *cmdPcapsplit) ParsingFinished() (err error) {
	if !p.shouldExecute {
		return
	}
	errs.CheckE(os.MkdirAll(p.DestDirName, 0755))
	splitter := pcapsplit.NewSplitter(p.TobBook)
	splitter.SetInput(p.InputFileName, p.PacketNumLimit)
	for _, o := range p.OptionIds {
		splitter.FilterAdd(o.OptionId)
	}
	errs.CheckE(splitter.AnalyzeInput())
	pbo := splitter.PacketByOptionAll()
	//log.Println(splitter.AllPacketOids())
	//log.Println(pbo)
	for oid, pnums := range pbo {
		if len(pnums) < p.MinPacketsPerOid {
			//log.Printf("option %s with %d packets ignored\n", oid, len(pnums))
			delete(pbo, oid)
		}
	}
	confs := make(map[packet.OptionId]pcapsplit.SplitByOptionsConfig)
	for oid, pnums := range pbo {
		log.Printf("oid %s => pkts %d : %v\n", oid, len(pnums), pnums)
		outFileName := fmt.Sprintf("%s/%s.pcap", p.DestDirName, oid)
		if p.UseEditcap {
			editcapArgs := []string{
				"-r",
				p.InputFileName,
				outFileName,
			}
			for _, pnum := range pnums {
				editcapArgs = append(editcapArgs, strconv.Itoa(pnum))
			}
			//log.Printf("editcap %v\n", editcapArgs)
			cmdStr := fmt.Sprintf("%v", editcapArgs)
			cmdStr = cmdStr[1 : len(cmdStr)-1]
			cmdStr = "editcap " + cmdStr
			fmt.Println(cmdStr)
		} else {
			outFile, err := os.Create(outFileName)
			errs.CheckE(err)
			defer func() { errs.CheckE(outFile.Close()) }()
			confs[oid] = pcapsplit.SplitByOptionsConfig{Writer: outFile}
		}
	}
	if !p.UseEditcap {
		errs.CheckE(splitter.SplitByOptions(confs))
	}
	return
}

func init() {
	var c cmdPcapsplit
	Registry.Register(&c)
}

// packet.OptionId wrapper supporting go-flags parsing
type optionId struct {
	packet.OptionId
}

func (o *optionId) UnmarshalFlag(value string) (err error) {
	defer errs.PassE(&err)
	v, err := strconv.ParseUint(value, 0, 64)
	errs.CheckE(err)
	o.OptionId = packet.OptionIdFromUint64(v)
	return
}
