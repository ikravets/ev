// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package exch

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/lunixbochs/struc"

	"my/errs"

	"my/itto/verify/exch/sbtcp"
)

type ExchangeSimulator interface {
	Run()
}

func NewExchangeSimulatorServer() ExchangeSimulator {
	return &exchange{
		glimpse: &glimpseServer{laddr: ":16001"},
		replay:  &replayServer{laddr: ":17001"},
		//mcast:   &mcastServer{laddr: "10.1.0.11:0", raddr: "233.54.12.1:18001"},
		mcast: &mcastServer{laddr: "10.2.0.5:0", raddr: "233.54.12.1:18001"},
	}
}

type exchange struct {
	glimpse *glimpseServer
	replay  *replayServer
	mcast   *mcastServer
}

func (e *exchange) Run() {
	go e.glimpse.run()
	go e.replay.run()
	go e.mcast.run()
	go log.Println("started")
	select {}
}

type glimpseServer struct {
	laddr string
}

func (s *glimpseServer) run() {
	l, err := net.Listen("tcp", s.laddr)
	errs.CheckE(err)
	defer l.Close()
	for {
		conn, err := l.Accept()
		errs.CheckE(err)
		log.Printf("accepted %s -> %s \n", conn.RemoteAddr(), conn.LocalAddr())
		go s.handleClient(conn)
	}
}

func (s *glimpseServer) handleClient(conn net.Conn) {
	defer conn.Close()
	m, err := sbtcp.ReadMessage(conn)
	errs.CheckE(err)
	log.Printf("got %#v\n", m)
	lr := m.(*sbtcp.MessageLoginRequest)
	errs.Check(lr != nil)

	la := sbtcp.MessageLoginAccepted{
		Session:        "00TestSess",
		SequenceNumber: 1,
	}
	errs.CheckE(sbtcp.WriteMessage(conn, &la))
	log.Printf("glimpse send: %v\n", la)

	seqNum := 5
	snap := fmt.Sprintf("M%020d", seqNum)
	sd := sbtcp.MessageSequencedData{}
	sd.SetPayload([]byte(snap))
	errs.CheckE(sbtcp.WriteMessage(conn, &sd))
	log.Printf("glimpse send: %v\n", sd)
}

type replayServer struct {
	laddr string
}

func (s *replayServer) run() {
	type moldudp64request struct {
		Session        string
		SequenceNumber uint64
		MessageCount   uint16
	}

	laddr, err := net.ResolveUDPAddr("udp", s.laddr)
	errs.CheckE(err)
	conn, err := net.ListenUDP("udp", laddr)
	errs.CheckE(err)
	defer conn.Close()
	buf := make([]byte, 20, 65536)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		errs.CheckE(err)
		if n != 20 {
			log.Printf("ignore wrong request from %s: %v\n", addr, buf)
			continue
		}
		req := &moldudp64request{
			Session:        string(buf[0:10]),
			SequenceNumber: binary.BigEndian.Uint64(buf[10:18]),
			MessageCount:   binary.BigEndian.Uint16(buf[18:20]),
		}
		log.Printf("got request: %v\n", req)
		resp := createPacket(int(req.SequenceNumber), 1)
		log.Printf("send response: %v\n", resp)

		//addr, err = net.ResolveUDPAddr("udp", "233.54.12.1:18001")
		//errs.CheckE(err)

		n, err = conn.WriteToUDP(resp, addr)
		errs.CheckE(err)
		errs.Check(n == len(resp), n, len(resp))
	}
}

type mcastServer struct {
	laddr string
	raddr string
}

func (s *mcastServer) run() {
	laddr, err := net.ResolveUDPAddr("udp", s.laddr)
	errs.CheckE(err)
	raddr, err := net.ResolveUDPAddr("udp", s.raddr)
	errs.CheckE(err)
	conn, err := net.DialUDP("udp", laddr, raddr)
	var seq int
	for {
		p := createPacket(seq, 1)
		log.Printf("send mcast: %v\n", p)
		n, err := conn.Write(p)
		errs.CheckE(err)
		errs.Check(n == len(p), n, len(p))

		time.Sleep(time.Second)
		seq++
		/*
			if seq > 30 {
				seq = 1
			}
		*/
	}
	defer conn.Close()
}

func createPacket(startSeqNum, count int) []byte {
	type moldUDP64 struct {
		Session        string `struc:"[10]byte"`
		SequenceNumber uint64
		MessageCount   uint16
	}
	type ittoMessageOptionsTrade struct {
		Type      byte
		Timestamp uint32
		Side      byte
		OId       uint32
		Cross     uint32
		Match     uint32
		Price     uint32
		Size      uint32
	}
	type moldUDP64MessageBlock struct {
		MessageLength uint16 //`struc:"sizeof=Payload"`
		Payload       ittoMessageOptionsTrade
	}

	errs.Check(startSeqNum >= 0)
	errs.Check(count >= 0 && count < 1000)
	mh := moldUDP64{
		Session:        "00TestSess",
		SequenceNumber: uint64(startSeqNum),
		MessageCount:   uint16(count),
	}
	var bb bytes.Buffer
	errs.CheckE(struc.Pack(&bb, &mh))
	for i := 0; i < count; i++ {
		mb := moldUDP64MessageBlock{
			MessageLength: 26,
			Payload: ittoMessageOptionsTrade{
				Type: 'P',
			},
		}
		errs.CheckE(struc.Pack(&bb, &mb))
	}
	return bb.Bytes()
}
