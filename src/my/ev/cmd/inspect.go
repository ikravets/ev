// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"io/ioutil"
	"my/ev/inspect"
	"my/ev/inspect/device"
	"os"

	"github.com/ikravets/errs"
	"github.com/jessevdk/go-flags"
)

type cmdInspect struct {
	ConfigFileName       string `long:"config" short:"c" required:"y" value-name:"YML_FILE" description:"input register config file to read"`
	OutputFileName       string `long:"output" short:"o" value-name:"FILE" default:"/dev/stdout" default-mask:"stdout" description:"output file"`
	OutputConfigFileName string `long:"outconfig" value-name:"FILE" description:"output updated config file"`
	shouldExecute        bool
}

func (c *cmdInspect) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdInspect) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("inspect", "read registers found in config file", "", c)
}

func init() {
	var c cmdInspect
	Registry.Register(&c)
}

func (c *cmdInspect) ParsingFinished() (err error) {
	if !c.shouldExecute {
		return
	}
	buf, err := ioutil.ReadFile(c.ConfigFileName)
	errs.CheckE(err)
	outFile, err := os.OpenFile(c.OutputFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	errs.CheckE(err)
	defer func() { errs.CheckE(outFile.Close()) }()
	dev, err := device.NewEfh_toolDevice()
	errs.CheckE(err)
	conf := inspect.NewConfig(dev)
	errs.CheckE(conf.Parse(string(buf)))
	errs.CheckE(conf.Probe())
	_, err = outFile.WriteString(conf.Report())
	errs.CheckE(err)

	if c.OutputConfigFileName != "" {
		newCfg, err := os.OpenFile(c.OutputConfigFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		errs.CheckE(err)
		defer func() { errs.CheckE(newCfg.Close()) }()
		dump, err := conf.Dump()
		errs.CheckE(err)
		_, err = newCfg.WriteString(dump)
		errs.CheckE(err)
	}

	return
}
