// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"io"
	"os"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/ikravets/errs"
)

type Replay interface {
	Run() error
	Stop()
	Progress() (done float64, ok bool)
}

type ReplayConfig struct {
	IfaceName string
	DumpName  string
	Limit     int
	Pps       int
	Loop      int
}

type replay struct {
	conf       ReplayConfig
	stopCh     chan struct{}
	progressCh chan float64
}

func NewReplay(c *ReplayConfig) Replay {
	return &replay{
		conf:       *c,
		stopCh:     make(chan struct{}),
		progressCh: make(chan float64, 1),
	}
}

func (r *replay) Run() (err error) {
	defer errs.PassE(&err)
	defer close(r.progressCh)
	progress, err := newProgress(r.conf.DumpName, r.conf.Loop, r.conf.Limit, r.progressCh)
	errs.CheckE(err)

	loop := r.conf.Loop
	if loop == 0 {
		loop = 1
	}
	for j := 0; j < loop; j++ {
		progress.startIteration()
		in, err := pcap.OpenOffline(r.conf.DumpName)
		errs.CheckE(err)
		defer in.Close()

		out, err := pcap.OpenLive(r.conf.IfaceName, 65536, false, pcap.BlockForever)
		errs.CheckE(err)
		defer out.Close()

		start := time.Now()
		for i := 0; i < r.conf.Limit || r.conf.Limit == 0; i++ {
			select {
			case <-r.stopCh:
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
			if r.conf.Pps != 0 {
				now := time.Now()
				expected := time.Duration(i) * time.Second / time.Duration(r.conf.Pps)
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

func (r *replay) Stop() {
	close(r.stopCh)
}

func (r *replay) Progress() (done float64, ok bool) {
	select {
	case done, ok = <-r.progressCh:
	default:
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
