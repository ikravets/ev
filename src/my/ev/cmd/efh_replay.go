// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"log"

	"github.com/ikravets/errs"
	"github.com/jessevdk/go-flags"

	"my/ev/channels"
	"my/ev/efh"
)

type cmdEfhReplay struct {
	InputFileName   string `long:"replay-dump" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	OutputInterface string `long:"replay-iface" required:"y" value-name:"IFACE" description:"output interface name"`
	Pps             int    `long:"replay-pps"   short:"p" value-name:"NUM" description:"packets per second"`
	Limit           int    `long:"replay-limit" short:"L" value-name:"NUM" description:"stop after NUM packets"`
	Loop            int    `long:"replay-loop"  short:"l" value-name:"NUM" description:"loop NUM times"`

	EfhLoglevel  int      `long:"efh-loglevel" default:"6"`
	EfhIgnoreGap bool     `long:"efh-ignore-gap"`
	EfhDump      string   `long:"efh-dump"`
	EfhSubscribe []string `long:"efh-subscribe"`
	EfhChannel   []string `long:"efh-channel"`
	EfhProf      bool     `long:"efh-prof"`

	TestEfh string `long:"test-efh" default:"/usr/libexec/test_efh"`
	Local   bool   `long:"local"`

	shouldExecute bool
}

func (c *cmdEfhReplay) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdEfhReplay) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("efh_replay", "run test_efh and replay pcap dump", "", c)
}

func (c *cmdEfhReplay) ParsingFinished() (err error) {
	if !c.shouldExecute {
		return
	}
	cc := channels.NewConfig()
	for _, s := range c.EfhChannel {
		errs.CheckE(cc.LoadFromStr(s))
	}
	conf := efh.ReplayConfig{
		InputFileName:   c.InputFileName,
		OutputInterface: c.OutputInterface,
		Pps:             c.Pps,
		Limit:           c.Limit,
		Loop:            c.Loop,
		EfhLoglevel:     c.EfhLoglevel,
		EfhIgnoreGap:    c.EfhIgnoreGap,
		EfhDump:         c.EfhDump,
		EfhSubscribe:    c.EfhSubscribe,
		EfhChannel:      cc,
		EfhProf:         c.EfhProf,
		TestEfh:         c.TestEfh,
		Local:           c.Local,
	}
	er := efh.NewEfhReplay(conf)
	err = er.Run()
	if err != nil {
		log.Printf("ERROR %s", err)
	}
	return
}

func init() {
	var c cmdEfhReplay
	Registry.Register(&c)
}
