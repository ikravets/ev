// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package pcapsplit

import (
	"log"

	"code.google.com/p/gopacket"

	"my/itto/verify/packet"
	"my/itto/verify/packet/itto"
	"my/itto/verify/sim"
)

type Splitter struct {
	packetNum     int
	idb           sim.IttoDb
	packetOids    []itto.OptionId
	invalidOidNum int
	allPacketOids [][]itto.OptionId
}

func NewSplitter() *Splitter {
	return &Splitter{
		idb: sim.NewIttoDb(),
	}
}

func (s *Splitter) HandlePacket(packet gopacket.Packet) {
	s.packetNum++
	if s.packetNum%10000 == 0 {
		log.Printf("stats packets:%d %#v invOid:%d\n", s.packetNum, s.idb.Stats(), s.invalidOidNum)
	}
	//fmt.Println(s.packetNum, packet)

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
	//if len(uniqOids) > 0 {
	//log.Println(uniqOids)
	//}
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
