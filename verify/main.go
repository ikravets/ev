// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package main

import (
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/kr/pretty"
	"my/itto/verify/pcap2log"
	"my/itto/verify/pcap2memh"
	"os"
)

func processArgs() {
	var opts struct {
		Verbose []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
	}

	parser := flags.NewParser(&opts, flags.PassDoubleDash|flags.HelpFlag|flags.IgnoreUnknown)

	pcap2memh.InitArgv(parser)
	pcap2log.InitArgv(parser)
	_, err := parser.Parse()
	/*
		pretty.Println(err)
		pretty.Println(args)
		pretty.Println(parser)
	*/
	if err != nil {
		perr := err.(*flags.Error)
		fmt.Println(perr.Message)
	}
}

func main() {
	os.Setenv("PATH", os.ExpandEnv("$HOME/my/proj/ekaline/esniff/wireshark/build/run:$HOME/wireshark/build/run:$PATH"))
	processArgs()
	_ = pretty.Print
}
