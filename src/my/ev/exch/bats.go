// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package exch

import (
	"log"
	"net"
	"time"

	"my/errs"

	"my/ev/exch/bats"
)

type MessageSource interface {
	CurrentSequence() int
	GetMessage(int) bats.Message
	Run()
	Stop()
}

func NewBatsExchangeSimulatorServer(c Config) (es ExchangeSimulator, err error) {
	errs.Check(c.Protocol == "bats")
	src := NewBatsMessageSource()
	es = &exchangeBats{
		src: src,
		spin: &spinServer{
			laddr: ":16002",
			src:   src,
		},
	}
	return
}

type exchangeBats struct {
	src  MessageSource
	spin *spinServer
}

func (e *exchangeBats) Run() {
	go e.src.Run()
	go e.spin.run()
	log.Println("started")
	select {}
}

type spinServer struct {
	laddr string
	src   MessageSource
}

func (s *spinServer) run() {
	l, err := net.Listen("tcp", s.laddr)
	errs.CheckE(err)
	defer l.Close()
	for {
		conn, err := l.Accept()
		errs.CheckE(err)
		log.Printf("accepted %s -> %s \n", conn.RemoteAddr(), conn.LocalAddr())
		c := NewSpinServerConn(conn, s.src)
		go c.run()
	}
}

type spinServerConn struct {
	conn  net.Conn
	bconn bats.Conn
	src   MessageSource
}

func NewSpinServerConn(conn net.Conn, src MessageSource) *spinServerConn {
	return &spinServerConn{
		conn:  conn,
		bconn: bats.NewConn(conn),
		src:   src,
	}
}

func (s *spinServerConn) run() {
	defer s.conn.Close()
	s.login()
	cancelSendImageAvail := make(chan struct{})
	defer close(cancelSendImageAvail)
	go s.sendImageAvail(cancelSendImageAvail)

	m, err := s.bconn.ReadMessage()
	errs.CheckE(err)
	req, ok := m.(*bats.MessageSpinRequest)
	errs.Check(ok)
	close(cancelSendImageAvail)

	seq := s.src.CurrentSequence()
	errs.Check(int(req.Sequence) <= seq, req.Sequence, seq)
	res := bats.MessageSpinResponse{
		Sequence: req.Sequence,
		Count:    uint32(seq) - req.Sequence + 1,
		Status:   bats.SpinStatusAccepted,
	}
	errs.CheckE(s.bconn.WriteMessage(&res))
	errs.CheckE(s.sendAll(int(req.Sequence), seq+1))
	res2 := bats.MessageSpinFinished{
		Sequence: req.Sequence,
	}
	errs.CheckE(s.bconn.WriteMessage(&res2))
}
func (s *spinServerConn) login() {
	m, err := s.bconn.ReadMessage()
	errs.CheckE(err)
	_, ok := m.(*bats.MessageLogin)
	errs.Check(ok)
	res := bats.MessageLoginResponse{
		Status: bats.LoginAccepted,
	}
	errs.CheckE(s.bconn.WriteMessage(&res))
}
func (s *spinServerConn) sendImageAvail(cancel <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	select {
	case _, _ = <-cancel:
		return
	case <-ticker.C:
		seq := s.src.CurrentSequence() - 10
		if seq > 0 {
			sia := bats.MessageSpinImageAvail{
				Sequence: uint32(seq),
			}
			errs.CheckE(s.bconn.WriteMessage(&sia))
		}
	}
}
func (s *spinServerConn) sendAll(start, end int) (err error) {
	defer errs.PassE(&err)
	for i := start; i < end; i++ {
		m := s.src.GetMessage(i)
		errs.CheckE(s.bconn.WriteMessage(m))
	}
	return
}

type batsMessageSource struct {
	curSeq int
	cancel chan struct{}
}

func NewBatsMessageSource() *batsMessageSource {
	return &batsMessageSource{
		cancel: make(chan struct{}),
	}
}
func (bms *batsMessageSource) Run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	select {
	case _, _ = <-bms.cancel:
		return
	case <-ticker.C:
	}
}
func (bms *batsMessageSource) Stop() {
	close(bms.cancel)
}

func (bms *batsMessageSource) CurrentSequence() int {
	return bms.curSeq
}

func (bms *batsMessageSource) GetMessage(seqNum int) bats.Message {
	m := bats.MessageAddOrder{
		TimeOffset: uint32(seqNum),
		OrderId:    uint64(seqNum),
		Price:      uint64(seqNum),
		Quantity:   10,
	}
	return &m
}
