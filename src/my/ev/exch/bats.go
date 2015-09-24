// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package exch

import (
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/ikravets/errs"

	"my/ev/bchan"
	"my/ev/exch/bats"
)

type MessageSource interface {
	CurrentSequence() int
	GetMessage(int) bats.Message
	Run()
	RunInteractive()
	Stop()
}

func NewBatsExchangeSimulatorServer(c Config) (es ExchangeSimulator, err error) {
	errs.Check(c.Protocol == "bats")
	src := NewBatsMessageSource()
	es = &exchangeBats{
		interactive: c.Interactive,
		src:         src,
		spin: &spinServer{
			laddr: ":16002",
			src:   src,
		},
		mcast: newBatsMcastServer("10.2.0.5:0", "224.0.131.2:30110", src),
	}
	return
}

type exchangeBats struct {
	interactive bool
	src         MessageSource
	spin        *spinServer
	mcast       *batsMcastServer
}

func (e *exchangeBats) Run() {
	if e.interactive {
		go e.src.RunInteractive()
	} else {
		go e.src.Run()
	}
	go e.spin.run()
	errs.CheckE(e.mcast.start())
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
	defer errs.Catch(func(ce errs.CheckerError) {
		log.Printf("caught %s\n", ce)
	})
	defer s.conn.Close()
	errs.CheckE(s.login())
	cancelSendImageAvail := make(chan struct{})
	defer func() {
		// close channel only if not already closed
		select {
		case _, ok := <-cancelSendImageAvail:
			if !ok {
				return
			}
		default:
		}
		close(cancelSendImageAvail)
	}()
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
	errs.CheckE(s.bconn.WriteMessageSimple(&res))
	errs.CheckE(s.sendAll(int(req.Sequence), seq+1))
	res2 := bats.MessageSpinFinished{
		Sequence: req.Sequence,
	}
	errs.CheckE(s.bconn.WriteMessageSimple(&res2))
}
func (s *spinServerConn) login() (err error) {
	defer errs.PassE(&err)
	m, err := s.bconn.ReadMessage()
	errs.CheckE(err)
	_, ok := m.(*bats.MessageLogin)
	errs.Check(ok)
	res := bats.MessageLoginResponse{
		Status: bats.LoginAccepted,
	}
	errs.CheckE(s.bconn.WriteMessageSimple(&res))
	log.Printf("login done")
	return
}
func (s *spinServerConn) sendImageAvail(cancel <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case _, _ = <-cancel:
			log.Printf("cancelled")
			return
		case <-ticker.C:
			seq := s.src.CurrentSequence() - 10
			if seq > 0 {
				sia := bats.MessageSpinImageAvail{
					Sequence: uint32(seq),
				}
				errs.CheckE(s.bconn.WriteMessageSimple(&sia))
			}
		}
	}
}
func (s *spinServerConn) sendAll(start, end int) (err error) {
	defer errs.PassE(&err)
	for i := start; i < end; i++ {
		m := s.src.GetMessage(i)
		errs.CheckE(s.bconn.WriteMessageSimple(m))
	}
	return
}

type batsMcastServer struct {
	laddr string
	raddr string
	src   *batsMessageSource

	cancel chan struct{}
	bmsc   *batsMessageSourceClient
	pw     bats.PacketWriter
	conn   net.Conn
}

func newBatsMcastServer(laddr, raddr string, src *batsMessageSource) *batsMcastServer {
	return &batsMcastServer{
		laddr:  laddr,
		raddr:  raddr,
		src:    src,
		cancel: make(chan struct{}),
	}
}
func (s *batsMcastServer) start() (err error) {
	defer errs.PassE(&err)
	laddr, err := net.ResolveUDPAddr("udp", s.laddr)
	errs.CheckE(err)
	raddr, err := net.ResolveUDPAddr("udp", s.raddr)
	errs.CheckE(err)
	s.conn, err = net.DialUDP("udp", laddr, raddr)
	errs.CheckE(err)
	bconn := bats.NewConn(s.conn)
	s.pw = bconn.GetPacketWriterUnsync()
	s.bmsc = s.src.NewClient()
	go s.run()
	return
}
func (s *batsMcastServer) run() {
	defer s.conn.Close()
	defer s.bmsc.Close()
	ch := s.bmsc.Chan()

	log.Printf("ready. source chan %v", ch)
	for {
		select {
		case _, _ = <-s.cancel:
			log.Printf("cancelled")
			return
		case seq := <-ch:
			log.Printf("mcast seq %d", seq)
			m := s.src.GetMessage(seq)
			s.pw.SyncStart()
			errs.CheckE(s.pw.SetSequence(seq))
			errs.CheckE(s.pw.WriteMessage(m))
			errs.CheckE(s.pw.Flush())
		}
	}
}

type batsMessageSource struct {
	curSeq int64
	cancel chan struct{}
	bchan  bchan.Bchan
	mps    int
}

func NewBatsMessageSource() *batsMessageSource {
	return &batsMessageSource{
		cancel: make(chan struct{}),
		bchan:  bchan.NewBchan(),
		mps:    1,
	}
}
func (bms *batsMessageSource) Run() {
	ticker := time.NewTicker(time.Duration(1000/bms.mps) * time.Millisecond)
	defer ticker.Stop()
	defer bms.bchan.Close()
	for seq := 100; ; seq++ {
		select {
		case _, _ = <-bms.cancel:
			return
		case <-ticker.C:
			bms.produce(seq)
		}
	}
}
func (bms *batsMessageSource) RunInteractive() {
	for {
		fmt.Printf("enter source seq: ")
		var seq int
		_, err := fmt.Scan(&seq)
		errs.CheckE(err)
		bms.produce(seq)
	}
}
func (bms *batsMessageSource) produce(seq int) {
	log.Printf("source seq %d", seq)
	atomic.StoreInt64(&bms.curSeq, int64(seq))
	select {
	case bms.bchan.ProducerChan() <- seq:
		log.Printf("produced")
	default:
	}
}
func (bms *batsMessageSource) Stop() {
	close(bms.cancel)
}
func (bms *batsMessageSource) CurrentSequence() int {
	return int(atomic.LoadInt64(&bms.curSeq))
}
func (bms *batsMessageSource) NewClient() *batsMessageSourceClient {
	c := &batsMessageSourceClient{
		bc: bms.bchan.NewConsumer(),
		ch: make(chan int),
	}
	go c.run()
	return c
}
func (bms *batsMessageSource) GetMessage(seqNum int) bats.Message {
	m := bats.MessageAddOrder{
		TimeOffset: uint32(seqNum),
		OrderId:    uint64(seqNum),
		Price:      uint64(seqNum),
		Side:       'B',
		Quantity:   10,
	}
	return &m
}

type batsMessageSourceClient struct {
	bc bchan.BchanConsumer
	ch chan int
}

func (c *batsMessageSourceClient) Chan() chan int {
	return c.ch
}
func (c *batsMessageSourceClient) run() {
	for val := range c.bc.Chan() {
		//log.Printf("forwarding value %v to chan %v", val, c.ch)
		c.ch <- val.(int)
	}
	close(c.ch)
}
func (c *batsMessageSourceClient) Close() {
	c.bc.Close()
}
