// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package main

import (
	"fmt"
	"log"
	"my/itto/verify/cmd"
	"os"
	"runtime/pprof"

	"github.com/jessevdk/go-flags"
)

func processArgs() {
	var opts struct {
		Verbose     []bool `short:"v" long:"verbose" description:"show verbose debug information"`
		LogFileName string `short:"l" long:"log" value-name:"FILE" default:"/dev/stderr" default-mask:"stderr" description:"log file"`
		ProfileCpu  string `long:"profile-cpu" value-name:"FILE"`
		ProfileMem  string `long:"profile-mem" value-name:"FILE"`
	}

	parser := flags.NewParser(&opts, flags.PassDoubleDash|flags.HelpFlag)
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

	if opts.ProfileCpu != "" {
		profFile, err := os.Create(opts.ProfileCpu)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(profFile)
		defer pprof.StopCPUProfile()
	}

	cmd.Registry.ParsingFinished()

	if opts.ProfileMem != "" {
		profFile, err := os.Create(opts.ProfileMem)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(profFile)
		profFile.Close()
	}
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	processArgs()
}
