// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"io"
	"os"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"

	"my/errs"
)

type Replay struct {
	IfaceName  string
	DumpName   string
	Limit      int
	Pps        int
	Loop       int
	StopCh     chan struct{}
	DoneCh     chan struct{}
	ProgressCh chan float64
}

func (r *Replay) Run() (err error) {
	defer errs.PassE(&err)
	defer close(r.DoneCh)
	progress, err := newProgress(r.DumpName, r.Loop, r.Limit, r.ProgressCh)
	errs.CheckE(err)

	loop := r.Loop
	if loop == 0 {
		loop = 1
	}
	for j := 0; j < loop; j++ {
		progress.startIteration()
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
			data, ci, err := in.ZeroCopyReadPacketData()
			if err == io.EOF {
				break
			}
			errs.CheckE(err)
			errs.CheckE(out.WritePacketData(data))
			progress.addPacket(ci)
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

type progress struct {
	totalIterations  int
	totalDumpSize    int
	totalPackets     int
	donePackets      int
	doneSizeApprox   int
	currentIteration int
	progressCh       chan<- float64
}

func newProgress(dumpFileName string, totalIterations int, limit int, progressCh chan<- float64) (p *progress, err error) {
	defer errs.PassE(&err)
	p = &progress{
		totalIterations: totalIterations,
		totalPackets:    limit,
		progressCh:      progressCh,
	}
	if p.totalIterations == 0 {
		p.totalIterations = 1
	}
	fi, err := os.Stat(dumpFileName)
	errs.CheckE(err)
	p.totalDumpSize = int(fi.Size())
	return
}
func (p *progress) startIteration() {
	if p.currentIteration == 1 {
		p.totalPackets = p.donePackets
	}
	p.donePackets = 0
	p.doneSizeApprox = 0
	p.currentIteration++
}
func (p *progress) addPacket(ci gopacket.CaptureInfo) {
	p.donePackets++
	p.doneSizeApprox += 16 + ci.CaptureLength
	p.emitProgress()
}
func (p *progress) emitProgress() {
	if len(p.progressCh) == cap(p.progressCh) {
		return
	}
	var done float64
	if p.totalPackets > 0 {
		done = float64(p.donePackets) / float64(p.totalPackets)
	} else if p.totalDumpSize > 0 {
		done = float64(p.doneSizeApprox) / float64(p.totalDumpSize)
	}
	done /= float64(p.totalIterations)
	select {
	case p.progressCh <- done:
	default:
	}
}
