// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package efhsim

import (
	"io"
	"log"

	"code.google.com/p/gopacket/pcap"

	"my/itto/verify/packet"
	"my/itto/verify/packet/processor"
	"my/itto/verify/sim"
)

type EfhSim struct {
	inputFileName    string
	inputPacketLimit int
	packetNum        int
	idb              sim.IttoDb
	book             sim.Book
	observer         *sim.MuxObserver
	subscr           *sim.Subscr
}

func NewEfhSim() *EfhSim {
	s := &EfhSim{
		idb:      sim.NewIttoDb(),
		book:     sim.NewBook(),
		observer: new(sim.MuxObserver),
		subscr:   sim.NewSubscr(),
	}
	s.idb.SetSubscription(s.subscr)
	return s
}

func (s *EfhSim) SetInput(fileName string, limit int) {
	s.inputFileName = fileName
	s.inputPacketLimit = limit
}
func (s *EfhSim) SubscribeFromReader(r io.Reader) error {
	return s.subscr.SubscribeFromReader(r)
}

func (s *EfhSim) AddLogger(logger sim.Observer) error {
	s.observer.AppendSlave(logger)
	return nil
}

func (s *EfhSim) AnalyzeInput() error {
	handle, err := pcap.OpenOffline(s.inputFileName)
	if err != nil {
		return err
	}
	defer handle.Close()
	pp := processor.NewCopyingProcessor()
	pp.LimitPacketNumber(s.inputPacketLimit)
	pp.SetObtainer(handle)
	pp.SetHandler(s)
	pp.ProcessAll()
	return nil
}

func (s *EfhSim) HandlePacket(packet packet.Packet) {
	s.packetNum++
	if s.packetNum%10000 == 0 {
		type Stats struct {
			Packets int
			Itto    sim.IttoDbStats
			Options int
		}
		s := Stats{
			Packets: s.packetNum,
			Itto:    s.idb.Stats(),
			Options: s.book.NumOptions(),
		}
		log.Printf("%#v", s)
	}
}

func (s *EfhSim) HandleMessage(message packet.ApplicationMessage) {
	//log.Println(message.Layer())
	m := s.idb.NewMessage(message)
	s.observer.MessageArrived(m)
	ops := s.idb.MessageOperations(m)
	for _, op := range ops {
		//log.Println(op)
		s.idb.ApplyOperation(op)
		s.observer.OperationAppliedToOrders(op)
		s.observer.BeforeBookUpdate(s.book, op)
		s.book.ApplyOperation(op)
		s.observer.AfterBookUpdate(s.book, op)
	}
}
