// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package efhsim

import (
	"io"
	"log"

	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/pcap"

	"my/itto/verify/packet"
	"my/itto/verify/sim"
)

type EfhSim struct {
	inputFileName    string
	inputPacketLimit int
	packetNum        int
	idb              sim.IttoDb
	book             sim.Book
	observer         *sim.MuxObserver
}

func NewEfhSim() *EfhSim {
	return &EfhSim{
		idb:      sim.NewIttoDb(),
		book:     sim.NewBook(),
		observer: new(sim.MuxObserver),
	}
}

func (s *EfhSim) SetInput(fileName string, limit int) {
	s.inputFileName = fileName
	s.inputPacketLimit = limit
}

func (s *EfhSim) OutSim(w io.Writer) error {
	s.observer.AppendSlave(sim.NewSimLogger(w))
	return nil
}

func (s *EfhSim) AnalyzeInput() error {
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
	return nil
}

func (s *EfhSim) HandlePacket(packet gopacket.Packet) {
	s.packetNum++
	if s.packetNum%10000 == 0 {
		log.Printf("stats packets:%d %#v\n", s.packetNum, s.idb.Stats())
	}
}

func (s *EfhSim) HandleMessage(message packet.ApplicationMessage) {
	//log.Println(message.Layer())
	m := sim.IttoDbMessage{Pam: message}
	s.observer.MessageArrived(&m)
	ops := s.idb.MessageOperations(&m)
	for _, op := range ops {
		//log.Println(op)
		s.idb.ApplyOperation(op)
		s.observer.OperationAppliedToOrders(op)
		s.observer.BeforeBookUpdate(s.book, op)
		s.book.ApplyOperation(op)
		s.observer.AfterBookUpdate(s.book, op)
	}
}
