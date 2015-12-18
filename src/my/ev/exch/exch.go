// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package exch

import (
	"errors"
)

type ExchangeSimulator interface {
	Run()
}

type Config struct {
	Protocol     string
	LocalAddr    string
	FeedAddr     string
	GapAddr      string
	Interactive  bool
	GapPeriod    uint64
	GapSize      uint64
	PartNumLimit int
	Speed        int
}

var IllegalProtocol = errors.New("Illegal protocol")

func NewExchangeSimulator(c Config) (es ExchangeSimulator, err error) {
	switch c.Protocol {
	case "nasdaq":
		es, err = NewNasdaqExchangeSimulatorServer(c)
	case "bats":
		es, err = NewBatsExchangeSimulatorServer(c)
	case "miax":
		es, err = NewMiaxExchangeSimulatorServer(c)
	default:
		err = IllegalProtocol
	}
	return
}
