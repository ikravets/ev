// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"fmt"
	"time"

	"github.com/google/gopacket/pcap"
	"github.com/jessevdk/go-flags"

	"my/errs"

	"my/ev/packet"
	"my/ev/packet/processor"
)

type cmdBenchmark struct {
	InputFileName string `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	ProcCopy      bool   `long:"proc-copy" short:"c" description:"use copying processor"`
	Iter          int    `long:"iter" short:"n" value-name:"NUM" default:"100" description:"number of iterations to run"`
	shouldExecute bool
}

func (c *cmdBenchmark) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdBenchmark) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("benchmark", "run benchmark", "", c)
}

func (c *cmdBenchmark) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	handle, err := pcap.OpenOffline(c.InputFileName)
	errs.CheckE(err)
	defer handle.Close()

	bo := packet.NewBufferedObtainer(handle)

	var pp packet.Processor
	if c.ProcCopy {
		pp = processor.NewCopyingProcessor()
	} else {
		pp = processor.NewReusingProcessor()
	}
	pp.SetObtainer(bo)

	var totalDuration time.Duration
	for i := 0; i < c.Iter; i++ {
		bo.Reset()
		start := time.Now()
		pp.ProcessAll()
		duration := time.Since(start)
		totalDuration += duration
	}
	timePerPacket := totalDuration / time.Duration(c.Iter*bo.Packets())
	fmt.Printf("total duration: %s, time/pkt: %s\n", totalDuration, timePerPacket)
}

func init() {
	var c cmdBenchmark
	Registry.Register(&c)
}
