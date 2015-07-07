// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"

	"github.com/jessevdk/go-flags"

	"my/errs"

	"my/ev/anal"
	"my/ev/efhsim"
	"my/ev/rec"
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
	Md5sum                  bool   `long:"md5sum" description:"compute md5sum on output file(s)"`
	shouldExecute           bool
	closers                 []io.Closer
}

func (c *cmdEfhsim) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdEfhsim) ConfigParser(parser *flags.Parser) {
	_, err := parser.AddCommand("efhsim", "simulate efh", "", c)
	errs.CheckE(err)
}

func (c *cmdEfhsim) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	efh := efhsim.NewEfhSim()
	efh.SetInput(c.InputFileName, c.PacketNumLimit)
	if c.SubscriptionFileName != "" {
		file, err := os.Open(c.SubscriptionFileName)
		errs.CheckE(err)
		errs.CheckE(efh.SubscribeFromReader(file))
		errs.CheckE(file.Close())
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
		defer errs.PassE(&err)
		var dict io.ReadCloser
		if c.InputFileNameAvtDict != "" {
			dict, err := os.Open(c.InputFileNameAvtDict)
			errs.CheckE(err)
			c.closers = append(c.closers, dict)
		}
		return efh.AddLogger(rec.NewAvtLogger(w, dict))
	})
	reporter := c.addAnalyzer(efh)

	// run efhsim
	errs.CheckE(efh.AnalyzeInput())

	for _, cl := range c.closers {
		errs.CheckE(cl.Close())
	}
	if reporter != nil {
		reporter.SaveAll()
	}
}

func (c *cmdEfhsim) addOut(fileName string, setOut func(io.Writer) error) {
	if fileName == "" {
		return
	}
	var err error
	var o io.WriteCloser
	if c.Md5sum {
		o, err = NewHashedOut(fileName)
	} else {
		o, err = os.Create(fileName)
	}
	errs.CheckE(err)
	errs.CheckE(setOut(o))
	c.closers = append(c.closers, o)
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

type hashedOut struct {
	file    *os.File
	md5file *os.File
	md5sum  hash.Hash
	mw      io.Writer
}

func NewHashedOut(baseName string) (o *hashedOut, err error) {
	defer errs.PassE(&err)
	o = &hashedOut{}
	o.file, err = os.Create(baseName)
	errs.CheckE(err)
	o.md5file, err = os.Create(baseName + ".md5sum")
	errs.CheckE(err)
	o.md5sum = md5.New()
	o.mw = io.MultiWriter(o.file, o.md5sum)
	return
}
func (o *hashedOut) writer() io.Writer {
	return o.mw
}
func (o *hashedOut) Write(b []byte) (int, error) {
	return o.mw.Write(b)
}
func (o *hashedOut) Close() (err error) {
	name := "-"
	defer errs.PassE(&err)
	if o.file != nil {
		errs.CheckE(o.file.Close())
		name = o.file.Name()
	}
	if o.md5file != nil {
		_, err := fmt.Fprintf(o.md5file, "%x\t%s\n", o.md5sum.Sum(nil), name)
		errs.CheckE(err)
		errs.CheckE(o.md5file.Close())
	}
	return
}
