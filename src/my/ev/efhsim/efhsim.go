// Copyright (c) Ilia Kravets, 2014-2016. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package efhsim

import (
	"io"
	"log"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/ikravets/errs"

	"my/ev/channels"
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

func NewEfhSim(shallow bool) *EfhSim {
	s := &EfhSim{
		simu:     sim.NewSim(shallow),
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
func (s *EfhSim) SubscriptionsNum() int {
	return s.simu.Subscr().Num()
}
func (s *EfhSim) RegisterChannels(cc channels.Config) (err error) {
	defer errs.PassE(&err)
	s.simu.SessionsIgnoreSrc(true)
	for _, c := range cc.Addrs() {
		a, err := net.ResolveUDPAddr("udp", c)
		errs.CheckE(err)
		ipFlow := gopacket.NewFlow(layers.EndpointIPv4, nil, a.IP.To4())
		portBytes := []byte{byte(a.Port >> 8), byte(a.Port)}
		udpFlow := gopacket.NewFlow(layers.EndpointUDPPort, nil, portBytes)
		s.simu.Session([]gopacket.Flow{ipFlow, udpFlow})
	}
	return
}

func (s *EfhSim) AddLogger(logger sim.Observer) error {
	s.observer.AppendSlave(logger)
	return nil
}

func (s *EfhSim) AnalyzeInput() (err error) {
	defer errs.PassE(&err)
	handle, err := pcap.OpenOffline(s.inputFileName)
	errs.CheckE(err)
	defer handle.Close()
	pp := processor.NewProcessor()
	pp.LimitPacketNumber(s.inputPacketLimit)
	pp.SetObtainer(handle)
	pp.SetHandler(s)
	errs.CheckE(pp.ProcessAll())
	return
}

func (s *EfhSim) HandlePacket(packet packet.Packet) {
	s.packetNum++
	if s.packetNum%10000 == 0 {
		type Stats struct {
			Packets      int
			OrderDbStats sim.OrderDbStats
			Options      int
			Sessions     int
		}
		s := Stats{
			Packets:      s.packetNum,
			OrderDbStats: s.simu.OrderDb().Stats(),
			Options:      s.simu.Book().NumOptions(),
			Sessions:     len(s.simu.Sessions()),
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
		if op.CanAffect(sim.OA_ORDERS) {
			s.simu.OrderDb().ApplyOperation(op)
			s.observer.OperationAppliedToOrders(op)
		}
		if op.CanAffect(sim.OA_BOOKS) {
			s.observer.BeforeBookUpdate(s.simu.Book(), op)
			s.simu.Book().ApplyOperation(op)
			s.observer.AfterBookUpdate(s.simu.Book(), op)
		}
	}
}
