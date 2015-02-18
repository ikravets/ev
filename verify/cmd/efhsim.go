// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"io"
	"log"
	"os"

	"github.com/jessevdk/go-flags"

	"my/itto/verify/efhsim"
	"my/itto/verify/sim"
)

type cmdEfhsim struct {
	InputFileName           string `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	OutputFileNameSim       string `long:"output-sim" value-name:"FILE" description:"output file for hw simulator"`
	OutputFileNameEfhOrders string `long:"output-efh-orders" value-name:"FILE" description:"output file for EFH order messages"`
	OutputFileNameEfhQuotes string `long:"output-efh-quotes" value-name:"FILE" description:"output file for EFH quote messages"`
	PacketNumLimit          int    `long:"count" short:"c" value-name:"NUM" description:"limit number of input packets"`
	shouldExecute           bool
	outFiles                []io.Closer
}

func (c *cmdEfhsim) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdEfhsim) ConfigParser(parser *flags.Parser) {
	_, err := parser.AddCommand("efhsim", "simulate efh", "", c)
	if err != nil {
		log.Fatalln(err)
	}
}

func (c *cmdEfhsim) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	defer func() {
		for _, f := range c.outFiles {
			f.Close()
		}
	}()
	efh := efhsim.NewEfhSim()
	efh.SetInput(c.InputFileName, c.PacketNumLimit)
	c.addOut(c.OutputFileNameSim, func(w io.Writer) error {
		return efh.AddLogger(sim.NewSimLogger(w))
	})
	c.addOut(c.OutputFileNameEfhOrders, func(w io.Writer) error {
		logger := efhsim.NewEfhLogger(w)
		logger.SetOutputMode(efhsim.EfhLoggerOutputOrders)
		return efh.AddLogger(logger)
	})
	c.addOut(c.OutputFileNameEfhQuotes, func(w io.Writer) error {
		logger := efhsim.NewEfhLogger(w)
		logger.SetOutputMode(efhsim.EfhLoggerOutputQuotes)
		return efh.AddLogger(logger)
	})
	if err := efh.AnalyzeInput(); err != nil {
		log.Fatal(err)
	}
}

func (c *cmdEfhsim) addOut(fileName string, setOut func(io.Writer) error) {
	if fileName == "" {
		return
	}
	file, err := os.Create(fileName)
	if err != nil {
		log.Fatalln(err)
	}
	if err := setOut(file); err != nil {
		file.Close()
		log.Fatalln(err)
	}
	c.outFiles = append(c.outFiles, file)
}

func init() {
	var c cmdEfhsim
	Registry.Register(&c)
}
