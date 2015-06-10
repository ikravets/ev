// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package main

import (
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/jessevdk/go-flags"

	"my/errs"

	"my/ev/cmd"
)

func main() {
	var err error
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	var opts struct {
		LogFileName string `short:"l" long:"log" value-name:"FILE" default:"/dev/stderr" default-mask:"stderr" description:"log file"`
		ProfileCpu  string `long:"profile-cpu" value-name:"FILE"`
		ProfileMem  string `long:"profile-mem" value-name:"FILE"`
	}

	parser := flags.NewParser(&opts, flags.PassDoubleDash|flags.HelpFlag)
	cmd.Registry.ConfigParser(parser)
	_, err = parser.Parse()
	if e, ok := err.(*flags.Error); ok && e.Type != flags.ErrUnknown {
		fmt.Printf("%s\n", e.Message)
		if e.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}
	errs.CheckE(err)

	logFile, err := os.OpenFile(opts.LogFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	errs.CheckE(err)
	log.SetOutput(logFile)
	defer logFile.Close()

	if opts.ProfileCpu != "" {
		profFile, err := os.Create(opts.ProfileCpu)
		errs.CheckE(err)
		pprof.StartCPUProfile(profFile)
		defer pprof.StopCPUProfile()
	}

	cmd.Registry.ParsingFinished()

	if opts.ProfileMem != "" {
		profFile, err := os.Create(opts.ProfileMem)
		errs.CheckE(err)
		pprof.WriteHeapProfile(profFile)
		profFile.Close()
	}
}
