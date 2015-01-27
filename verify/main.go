// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package main

import (
	"fmt"
	"log"
	"my/itto/verify/cmd"
	"os"

	"github.com/jessevdk/go-flags"
)

func processArgs() {
	var opts struct {
		Verbose     []bool `short:"v" long:"verbose" description:"show verbose debug information"`
		LogFileName string `short:"l" long:"log" value-name:"FILE" default:"/dev/stderr" default-mask:"stderr" description:"log file"`
	}

	parser := flags.NewParser(&opts, flags.PassDoubleDash|flags.HelpFlag|flags.IgnoreUnknown)
	cmd.Registry.ConfigParser(parser)
	if _, err := parser.Parse(); err != nil {
		fmt.Println(err.(*flags.Error).Message)
		os.Exit(1)
	}
	logFile, err := os.OpenFile(opts.LogFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(logFile)
	defer logFile.Close()
	cmd.Registry.ParsingFinished()
}

func main() {
	os.Setenv("PATH", os.ExpandEnv("$HOME/my/proj/ekaline/esniff/wireshark/build/run:$HOME/wireshark/build/run:$PATH"))
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	processArgs()
}
