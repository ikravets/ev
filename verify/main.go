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

func processArgs() (commands []func() error, finisher func()) {
	var opts struct {
		Verbose []bool `short:"v" long:"verbose" description:"show verbose debug information"`
	}

	var cmds []func() error
	finisher = func() {}

	parser := flags.NewParser(&opts, flags.PassDoubleDash|flags.HelpFlag|flags.IgnoreUnknown)
	cmds = append(cmds, pcap2memh.InitArgv(parser))
	cmds = append(cmds, pcap2log.InitArgv(parser))
	if _, err := parser.Parse(); err != nil {
		fmt.Println(err.(*flags.Error).Message)
		os.Exit(1)
	}
	commands = cmds
	return
}

func main() {
	os.Setenv("PATH", os.ExpandEnv("$HOME/my/proj/ekaline/esniff/wireshark/build/run:$HOME/wireshark/build/run:$PATH"))
	commands, finisher := processArgs()
	defer finisher()
	for _, c := range commands {
		c()
	}
	_ = pretty.Print
}
