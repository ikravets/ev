// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package pcapsplit

import (
	"io"
	"log"
	"os"

	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/layers"
	"code.google.com/p/gopacket/pcap"
	"code.google.com/p/gopacket/pcapgo"

	"my/itto/verify/packet"
	"my/itto/verify/packet/itto"
	"my/itto/verify/sim"
)

type Splitter struct {
	inputFileName    string
	inputPacketLimit int
	packetNum        int
	idb              sim.IttoDb
	packetOids       []itto.OptionId
	invalidOidNum    int
	allPacketOids    [][]itto.OptionId
}

func NewSplitter() *Splitter {
	return &Splitter{
		idb: sim.NewIttoDb(),
	}
}

func (s *Splitter) SetInput(fileName string, limit int) {
	s.inputFileName = fileName
	s.inputPacketLimit = limit
}

func (s *Splitter) AnalyzeInput() error {
	handle, err := pcap.OpenOffline(s.inputFileName)
	if err != nil {
		return err
	}
	defer handle.Close()
	pp := packet.NewProcessor()
	pp.LimitPacketNumber(s.inputPacketLimit)
	pp.SetObtainer(handle)
	pp.SetHandler(s)
	pp.ProcessAll()
	s.HandlePacket(nil) // process the last packet
	return nil
}

type SplitByOptionsConfig struct {
	Writer        io.Writer
	LastPacketNum *int
	AppendOnly    bool
}

func (s *Splitter) SplitByOptions(confs map[itto.OptionId]SplitByOptionsConfig) error {
	inHandle, err := pcap.OpenOffline(s.inputFileName)
	if err != nil {
		return err
	}
	defer inHandle.Close()

	type state struct {
		SplitByOptionsConfig
		w             *pcapgo.Writer
		lastPacketNum *int
	}
	states := make(map[itto.OptionId]*state)

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
			if err := state.w.WriteFileHeader(65536, layers.LinkTypeEthernet); err != nil {
				return err
			}
		}
		states[oid] = &state
	}

	for i, poids := range s.allPacketOids[1:] {
		data, ci, err := inHandle.ZeroCopyReadPacketData()
		if err != nil {
			return err
		}
		for _, poid := range poids {
			if state := states[poid]; state != nil && *state.lastPacketNum < i {
				if err := state.w.WritePacket(ci, data); err != nil {
					return err
				}
				*state.lastPacketNum = i
			}
		}
	}
	return nil
}

func (s *Splitter) SplitByOption(oid itto.OptionId, fileName string) error {
	outFile, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer outFile.Close()
	confs := make(map[itto.OptionId]SplitByOptionsConfig)
	confs[oid] = SplitByOptionsConfig{Writer: outFile}
	return s.SplitByOptions(confs)
}

func (s *Splitter) HandlePacket(packet gopacket.Packet) {
	s.packetNum++
	if s.packetNum%10000 == 0 {
		log.Printf("stats packets:%d %#v invOid:%d\n", s.packetNum, s.idb.Stats(), s.invalidOidNum)
	}
	// process *previous* packet info

	// uniquefy packet oids
	oidSet := make(map[itto.OptionId]struct{})
	uniq := 0
	for _, oid := range s.packetOids {
		if _, ok := oidSet[oid]; !ok {
			oidSet[oid] = struct{}{}
			s.packetOids[uniq] = oid
			uniq++
		}
	}
	uniqOids := make([]itto.OptionId, uniq)
	copy(uniqOids, s.packetOids)

	s.allPacketOids = append(s.allPacketOids, uniqOids)
	s.packetOids = s.packetOids[:0]
}

func (s *Splitter) HandleMessage(message packet.ApplicationMessage) {
	//log.Println(message.Layer())
	m := sim.IttoDbMessage{Pam: message}
	//s.idb.AddMessage(&m)
	ops := s.idb.MessageOperations(&m)
	for _, op := range ops {
		//log.Println(op)
		s.idb.ApplyOperation(op)
		oid := op.GetOptionId()
		if oid.Valid() {
			s.packetOids = append(s.packetOids, oid)
			//log.Println(oid)
		} else {
			//log.Println("invalid oid for op", op)
			s.invalidOidNum++
		}
	}
}

func (s *Splitter) AllPacketOids() [][]itto.OptionId {
	return s.allPacketOids
}

func (s *Splitter) PacketByOption(oid itto.OptionId) []int {
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

func (s *Splitter) PacketByOptionAll() map[itto.OptionId][]int {
	m := make(map[itto.OptionId][]int)
	for i, poids := range s.allPacketOids {
		for _, poid := range poids {
			m[poid] = append(m[poid], i)
		}
	}
	return m
}
