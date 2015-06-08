// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"encoding/binary"
	"hash/crc32"
	"io"
	"log"
	"os"

	"github.com/jessevdk/go-flags"
	"my/errs"

	"my/itto/verify/anal"
	"my/itto/verify/efhsim"
	"my/itto/verify/rec"
)

type cmdEfhsim struct {
	InputFileName           string `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	SubscriptionFileName    string `long:"subscribe" short:"s" value-name:"SUBSCRIPTION_FILE" description:"read subscriptions from file"`
	OutputFileNameSimOrders string `long:"output-sim-orders" value-name:"FILE" description:"output file for hw simulator"`
	OutputFileNameSimQuotes string `long:"output-sim-quotes" value-name:"FILE" description:"output file for hw simulator"`
	OutputFileNameEfhOrders string `long:"output-efh-orders" value-name:"FILE" description:"output file for EFH order messages"`
	OutputFileNameEfhQuotes string `long:"output-efh-quotes" value-name:"FILE" description:"output file for EFH quote messages"`
	OutputFileNameAvt       string `long:"output-avt" value-name:"FILE" description:"output file for AVT CSV"`
	InputFileNameAvtDict    string `long:"avt-dict" value-name:"DICT" description:"read dictionary for AVT CSV output"`
	OutputDirStats          string `long:"output-stats" value-name:"DIR" description:"output dir for stats"`
	PacketNumLimit          int    `long:"count" short:"c" value-name:"NUM" description:"limit number of input packets"`
	NoHwLim                 bool   `long:"no-hw-lim" description:"do not enforce HW limits"`
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
	if c.SubscriptionFileName != "" {
		file, err := os.Open(c.SubscriptionFileName)
		errs.CheckE(err)
		errs.CheckE(efh.SubscribeFromReader(file))
		file.Close()
	}
	if !c.NoHwLim {
		efh.AddLogger(rec.NewHwLimChecker())
	}
	c.addOut(c.OutputFileNameSimOrders, func(w io.Writer) error {
		logger := rec.NewSimLogger(w)
		logger.SetOutputMode(rec.EfhLoggerOutputOrders)
		return efh.AddLogger(logger)
	})
	c.addOut(c.OutputFileNameSimQuotes, func(w io.Writer) error {
		logger := rec.NewSimLogger(w)
		logger.SetOutputMode(rec.EfhLoggerOutputQuotes)
		return efh.AddLogger(logger)
	})
	c.addOut(c.OutputFileNameEfhOrders, func(w io.Writer) error {
		logger := rec.NewEfhLogger(rec.NewTestefhPrinter(w))
		logger.SetOutputMode(rec.EfhLoggerOutputOrders)
		return efh.AddLogger(logger)
	})
	c.addOut(c.OutputFileNameEfhQuotes, func(w io.Writer) error {
		logger := rec.NewEfhLogger(rec.NewTestefhPrinter(w))
		logger.SetOutputMode(rec.EfhLoggerOutputQuotes)
		return efh.AddLogger(logger)
	})
	c.addOut(c.OutputFileNameAvt, func(w io.Writer) (err error) {
		var dict io.ReadCloser
		if c.InputFileNameAvtDict != "" {
			if dict, err = os.Open(c.InputFileNameAvtDict); err != nil {
				return
			} else {
				c.outFiles = append(c.outFiles, dict)
			}
		}
		return efh.AddLogger(rec.NewAvtLogger(w, dict))
	})

	reporter := c.addAnalyzer(efh)
	errs.CheckE(efh.AnalyzeInput())
	if reporter != nil {
		reporter.SaveAll()
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
func (c *cmdEfhsim) addAnalyzer(efh *efhsim.EfhSim) *anal.Reporter {
	if c.OutputDirStats == "" {
		return nil
	}
	hashFn := func(v uint64) uint64 {
		data := make([]byte, 8)
		binary.BigEndian.PutUint64(data, v)
		h := crc32.ChecksumIEEE(data)
		return uint64(h & (1<<24 - 1))
	}
	moduloFn := func(v uint64) uint64 {
		return v & (1<<24 - 1)
	}
	analyzer := anal.NewAnalyzer()
	analyzer.AddOrderHashFunction(hashFn)
	analyzer.AddOrderHashFunction(moduloFn)
	efh.AddLogger(analyzer.Observer())
	reporter := anal.NewReporter()
	reporter.SetAnalyzer(analyzer)
	reporter.SetOutputDir(c.OutputDirStats)
	return reporter
}

func init() {
	var c cmdEfhsim
	Registry.Register(&c)
}
