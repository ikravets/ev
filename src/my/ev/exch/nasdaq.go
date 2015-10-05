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

	"github.com/ikravets/errs"
	"github.com/lunixbochs/struc"

	"my/ev/exch/sbtcp"
)

func NewNasdaqExchangeSimulatorServer(c Config) (es ExchangeSimulator, err error) {
	errs.Check(c.Protocol == "nasdaq")
	errs.Check(!c.Interactive)
	es = &exchangeNasdaq{
		glimpse: &glimpseServer{
			laddr:          ":16001",
			snapshotSeqNum: 5,
		},
		replay: &replayServer{
			laddr:        ":17001",
			sleepEnabled: true,
		},
		mcast: &mcastServer{
			laddr: c.LocalAddr,
			raddr: c.RemoteAddr,
			seq:   1000,
			pps:   1,
		},
	}
	return
}

type exchangeNasdaq struct {
	glimpse *glimpseServer
	replay  *replayServer
	mcast   *mcastServer
}

func (e *exchangeNasdaq) Run() {
	go e.glimpse.run()
	go e.replay.run()
	go e.mcast.run()
	log.Println("started")
	select {}
}

type glimpseServer struct {
	laddr          string
	snapshotSeqNum int
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

	for i := 0; i < s.snapshotSeqNum; i++ {
		s.sendSeqData(conn, generateIttoMessage(i))
	}
	snap := fmt.Sprintf("M%020d", s.snapshotSeqNum)
	s.sendSeqData(conn, []byte(snap))
}
func (s *glimpseServer) sendSeqData(conn net.Conn, data []byte) {
	sd := sbtcp.MessageSequencedData{}
	sd.SetPayload(data)
	errs.CheckE(sbtcp.WriteMessage(conn, &sd))
	log.Printf("glimpse send: %v\n", sd)
}

type replayServer struct {
	laddr        string
	sleepEnabled bool
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
		go func() {
			const MAX_MESSAGES = (1500 - 34 - 20) / 40
			log.Printf("got request: %v\n", req)
			//num := int(req.SequenceNumber) - 10
			num := int(req.MessageCount)
			if num <= 0 {
				num = 1
			} else if num > MAX_MESSAGES {
				num = MAX_MESSAGES
			}
			resp, err := createMoldPacket(int(req.SequenceNumber), num)
			errs.CheckE(err)
			errs.Check(len(resp) < 1500-34)

			if s.sleepEnabled {
				sleep := time.Duration(250+2500/num) * time.Millisecond
				log.Printf("sleeping for %s\n", sleep)
				time.Sleep(sleep)
			}
			log.Printf("send response: %v\n", resp)
			n, err = conn.WriteToUDP(resp, addr)
			errs.CheckE(err)
			errs.Check(n == len(resp), n, len(resp))
		}()
	}
}

type mcastServer struct {
	laddr string
	raddr string
	seq   int
	pps   int
}

func (s *mcastServer) run() {
	errs.Check(s.pps != 0)
	laddr, err := net.ResolveUDPAddr("udp", s.laddr)
	errs.CheckE(err)
	raddr, err := net.ResolveUDPAddr("udp", s.raddr)
	errs.CheckE(err)
	conn, err := net.DialUDP("udp", laddr, raddr)
	errs.CheckE(err)
	for {
		p, err := createMoldPacket(s.seq, 1)
		errs.CheckE(err)
		log.Printf("send mcast: %v\n", p)
		n, err := conn.Write(p)
		errs.CheckE(err)
		errs.Check(n == len(p), n, len(p))

		delay := time.Duration(1000/s.pps) * time.Millisecond
		time.Sleep(delay)
		s.seq++
	}
	defer conn.Close()
}

func createMoldPacket(startSeqNum, count int) (bs []byte, err error) {
	defer errs.PassE(&err)
	type moldUDP64 struct {
		Session        string `struc:"[10]byte"`
		SequenceNumber uint64
		MessageCount   uint16
	}
	type moldUDP64MessageBlock struct {
		MessageLength int16 `struc:"sizeof=Payload"`
		Payload       []uint8
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
			Payload: generateIttoMessage(startSeqNum + i),
		}
		errs.CheckE(struc.Pack(&bb, &mb))
	}
	bs = bb.Bytes()
	return
}

func generateIttoMessage(seqNum int) []byte {
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
	type ittoMessageOptionDirectory struct {
		Type             byte
		Timestamp        uint32
		OId              uint32
		Symbol           [6]byte
		ExpirationYear   byte
		ExpirationMonth  byte
		ExpirationDay    byte
		StrikePrice      uint32
		OType            byte
		Source           uint8
		UnderlyingSymbol [13]byte
		ClosingType      byte
		Tradable         byte
		MPV              byte
	}
	type ittoMessageAddOrder struct {
		Type      byte
		Timestamp uint32
		RefNumD   uint32
		Side      byte
		OId       uint32
		Price     uint32
		Size      uint32
	}

	errs.Check(seqNum >= 0)
	var bb bytes.Buffer
	switch seqNum % 3 {
	case 0:
		m := ittoMessageOptionDirectory{
			Type: 'R',
			OId:  uint32(seqNum / 3),
		}
		errs.CheckE(struc.Pack(&bb, &m))
	case 1:
		m := ittoMessageOptionsTrade{
			Type: 'P',
			OId:  uint32(seqNum / 3),
		}
		errs.CheckE(struc.Pack(&bb, &m))
	case 2:
		m := ittoMessageAddOrder{
			Type:    'A',
			RefNumD: uint32(seqNum),
			Side:    'B',
			OId:     uint32(seqNum/9) + 0x10000,
			Price:   uint32(seqNum % 10 * 1000),
			Size:    uint32(seqNum % 10),
		}
		errs.CheckE(struc.Pack(&bb, &m))
	}
	return bb.Bytes()
}
