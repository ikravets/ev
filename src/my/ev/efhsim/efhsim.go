// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package efhsim

import (
	"io"
	"log"

	"github.com/google/gopacket/pcap"

	"my/ev/packet"
	"my/ev/packet/processor"
	"my/ev/sim"
)

type EfhSim struct {
	inputFileName    string
	inputPacketLimit int
	packetNum        int
	simu             sim.Sim
	observer         *sim.MuxObserver
}

func NewEfhSim() *EfhSim {
	s := &EfhSim{
		simu:     sim.NewSim(),
		observer: sim.NewMuxObserver(),
	}
	return s
}

func (s *EfhSim) SetInput(fileName string, limit int) {
	s.inputFileName = fileName
	s.inputPacketLimit = limit
}
func (s *EfhSim) SubscribeFromReader(r io.Reader) error {
	return s.simu.Subscr().SubscribeFromReader(r)
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
	pp := processor.NewProcessor()
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
			Packets      int
			OrderDbStats sim.OrderDbStats
			Options      int
		}
		s := Stats{
			Packets:      s.packetNum,
			OrderDbStats: s.simu.OrderDb().Stats(),
			Options:      s.simu.Book().NumOptions(),
		}
		log.Printf("%#v", s)
	}
}

func (s *EfhSim) HandleMessage(message packet.ApplicationMessage) {
	//log.Println(message.Layer())
	m := s.simu.NewMessage(message)
	s.observer.MessageArrived(m)
	ops := m.MessageOperations()
	for _, op := range ops {
		//log.Println(op)
		s.simu.OrderDb().ApplyOperation(op)
		s.observer.OperationAppliedToOrders(op)
		s.observer.BeforeBookUpdate(s.simu.Book(), op)
		s.simu.Book().ApplyOperation(op)
		s.observer.AfterBookUpdate(s.simu.Book(), op)
	}
}
