// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"io"
	"time"

	"github.com/google/gopacket/pcap"

	"my/errs"
)

type Replay struct {
	IfaceName string
	DumpName  string
	Limit     int
	Pps       int
	Loop      int
	StopCh    chan struct{}
	DoneCh    chan struct{}
}

func (r *Replay) Run() (err error) {
	defer errs.PassE(&err)
	defer close(r.DoneCh)

	loop := r.Loop
	if loop == 0 {
		loop = 1
	}
	for j := 0; j < loop; j++ {
		in, err := pcap.OpenOffline(r.DumpName)
		errs.CheckE(err)
		defer in.Close()

		out, err := pcap.OpenLive(r.IfaceName, 65536, false, pcap.BlockForever)
		errs.CheckE(err)
		defer out.Close()

		start := time.Now()
		for i := 0; i < r.Limit || r.Limit == 0; i++ {
			select {
			case <-r.StopCh:
				return nil
			default:
			}
			data, _, err := in.ZeroCopyReadPacketData()
			if err == io.EOF {
				break
			}
			errs.CheckE(err)
			errs.CheckE(out.WritePacketData(data))
			if r.Pps != 0 {
				now := time.Now()
				expected := time.Duration(i) * time.Second / time.Duration(r.Pps)
				actual := now.Sub(start)
				diff := expected - actual
				if diff > 0 {
					time.Sleep(diff)
				}
			}
		}
	}
	return
}
