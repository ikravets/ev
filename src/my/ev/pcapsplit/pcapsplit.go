// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package pcapsplit

import (
	"io"
	"log"
	"os"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"

	"my/errs"

	"my/ev/packet"
	"my/ev/packet/processor"
	"my/ev/sim"
)

type Splitter struct {
	inputFileName    string
	inputPacketLimit int
	simu             sim.Sim
	packetOids       []packet.OptionId
	allPacketOids    [][]packet.OptionId
	oidFilter        map[packet.OptionId]struct{}
	stats            SplitterStats
}

func NewSplitter() *Splitter {
	return &Splitter{
		simu: sim.NewSim(),
	}
}

func (s *Splitter) SetInput(fileName string, limit int) {
	s.inputFileName = fileName
	s.inputPacketLimit = limit
}

func (s *Splitter) AnalyzeInput() (err error) {
	defer errs.PassE(&err)
	handle, err := pcap.OpenOffline(s.inputFileName)
	errs.CheckE(err)
	defer handle.Close()
	pp := processor.NewProcessor()
	pp.LimitPacketNumber(s.inputPacketLimit)
	pp.SetObtainer(handle)
	pp.SetHandler(s)
	errs.CheckE(pp.ProcessAll())
	s.HandlePacket(nil) // process the last packet
	return
}

type SplitByOptionsConfig struct {
	Writer        io.Writer
	LastPacketNum *int
	AppendOnly    bool
}

func (s *Splitter) SplitByOptions(confs map[packet.OptionId]SplitByOptionsConfig) (err error) {
	defer errs.PassE(&err)
	inHandle, err := pcap.OpenOffline(s.inputFileName)
	errs.CheckE(err)
	defer inHandle.Close()

	type state struct {
		SplitByOptionsConfig
		w             *pcapgo.Writer
		lastPacketNum *int
	}
	states := make(map[packet.OptionId]*state)

	for oid, conf := range confs {
		state := state{
			SplitByOptionsConfig: conf,
			w:                    pcapgo.NewWriter(conf.Writer),
			lastPacketNum:        conf.LastPacketNum,
		}
		if state.lastPacketNum == nil {
			state.lastPacketNum = new(int)
		}
		if !conf.AppendOnly {
			errs.CheckE(state.w.WriteFileHeader(65536, layers.LinkTypeEthernet))
		}
		states[oid] = &state
	}

	for i, poids := range s.allPacketOids[1:] {
		data, ci, err := inHandle.ZeroCopyReadPacketData()
		errs.CheckE(err)
		for _, poid := range poids {
			if state := states[poid]; state != nil && *state.lastPacketNum < i {
				errs.CheckE(state.w.WritePacket(ci, data))
				*state.lastPacketNum = i
			}
		}
	}
	return nil
}

func (s *Splitter) SplitByOption(oid packet.OptionId, fileName string) (err error) {
	defer errs.PassE(&err)
	outFile, err := os.Create(fileName)
	errs.CheckE(err)
	defer func() { errs.CheckE(outFile.Close()) }()
	confs := make(map[packet.OptionId]SplitByOptionsConfig)
	confs[oid] = SplitByOptionsConfig{Writer: outFile}
	return s.SplitByOptions(confs)
}

func (s *Splitter) HandlePacket(_ packet.Packet) {
	s.stats.Packets++
	if s.stats.Packets%10000 == 0 {
		log.Printf("stats: %#v %#v\n", s.stats, s.simu.OrderDb().Stats())
	}
	// process *previous* packet info

	// uniquefy packet oids
	oidSet := make(map[packet.OptionId]struct{})
	uniq := 0
	for _, oid := range s.packetOids {
		if _, ok := oidSet[oid]; !ok {
			oidSet[oid] = struct{}{}
			s.packetOids[uniq] = oid
			uniq++
		}
	}
	uniqOids := make([]packet.OptionId, uniq)
	copy(uniqOids, s.packetOids)

	s.allPacketOids = append(s.allPacketOids, uniqOids)
	s.packetOids = s.packetOids[:0]
}

func (s *Splitter) HandleMessage(message packet.ApplicationMessage) {
	//log.Println(message.Layer())
	m := s.simu.NewMessage(message)
	ops := m.MessageOperations()
	for _, op := range ops {
		s.stats.Operations++
		oid := op.GetOptionId()
		if oid.Valid() {
			if s.acceptOid(oid) {
				s.packetOids = append(s.packetOids, oid)
				s.simu.OrderDb().ApplyOperation(op)
				s.simu.Book().ApplyOperation(op)
			} else {
				s.stats.IgnoredOperations++
			}
		} else {
			s.stats.InvalidOidOps++
		}
	}
}

func (s *Splitter) Filter(oids []packet.OptionId) {
	for _, oid := range oids {
		s.FilterAdd(oid)
	}
}
func (s *Splitter) FilterAdd(oid packet.OptionId) {
	if s.oidFilter == nil {
		s.oidFilter = make(map[packet.OptionId]struct{})
	}
	s.oidFilter[oid] = struct{}{}
}
func (s *Splitter) acceptOid(oid packet.OptionId) bool {
	if s.oidFilter == nil {
		return true
	}
	_, ok := s.oidFilter[oid]
	return ok
}

func (s *Splitter) AllPacketOids() [][]packet.OptionId {
	return s.allPacketOids
}

func (s *Splitter) PacketByOption(oid packet.OptionId) []int {
	var pnums []int
	for i, poids := range s.allPacketOids {
		for _, poid := range poids {
			if poid == oid {
				pnums = append(pnums, i)
			}
		}
	}
	return pnums
}

func (s *Splitter) PacketByOptionAll() map[packet.OptionId][]int {
	m := make(map[packet.OptionId][]int)
	for i, poids := range s.allPacketOids {
		for _, poid := range poids {
			m[poid] = append(m[poid], i)
		}
	}
	return m
}

type SplitterStats struct {
	Packets           int
	InvalidOidOps     int
	Operations        int
	IgnoredOperations int
}

func (s *Splitter) Stats() SplitterStats {
	return s.stats
}
