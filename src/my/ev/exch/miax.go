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
	"my/ev/exch/miax"
)

type MiaxMessageSource interface {
	SetSequence(int)
	CurrentSequence() int
	GetMessage(int) miax.Message
	Run()
	RunInteractive()
	Stop()
}

func NewMiaxExchangeSimulatorServer(c Config) (es ExchangeSimulator, err error) {
	errs.Check(c.Protocol == "miax")
	src := NewMiaxMessageSource()
	es = &exchangeMiax{
		interactive: c.Interactive,
		src:         src,
		sesm: &SesMServer{
			laddr: ":16002",
			src:   src,
		},
		mcast: newMiaxMcastServer("10.2.0.5:0", "224.0.131.2:30110", src),
	}
	return
}

type exchangeMiax struct {
	interactive bool
	src         MiaxMessageSource
	sesm        *SesMServer
	mcast       *miaxMcastServer
}

func (e *exchangeMiax) Run() {
	if e.interactive {
		go e.src.RunInteractive()
	} else {
		go e.src.Run()
	}
	go e.sesm.run()
	errs.CheckE(e.mcast.start())
	log.Println("started")
	select {}
}

type SesMServer struct {
	laddr string
	src   *miaxMessageSource
}

func (s *SesMServer) run() {
	l, err := net.Listen("tcp", s.laddr)
	errs.CheckE(err)
	defer l.Close()
	for {
		conn, err := l.Accept()
		errs.CheckE(err)
		log.Printf("accepted %s -> %s \n", conn.RemoteAddr(), conn.LocalAddr())
		c := NewSesMServerConn(conn, s.src)
		go c.run()
	}
}

type SesMServerConn struct {
	conn            net.Conn
	bconn           miax.Conn
	src             *miaxMessageSource
	imageLag        int
	mcastDuringSesM int
}

func NewSesMServerConn(conn net.Conn, src *miaxMessageSource) *SesMServerConn {
	return &SesMServerConn{
		conn:            conn,
		bconn:           miax.NewConn(conn),
		src:             src,
		imageLag:        10,
		mcastDuringSesM: 10,
	}
}

func (s *SesMServerConn) run() {
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
	req, ok := m.(*miax.MessageSesMRequest)
	errs.Check(ok)
	close(cancelSendImageAvail)

	seq := s.src.CurrentSequence()
	errs.Check(int(req.Sequence) <= seq, req.Sequence, seq)
	res := miax.MessageSesMResponse{
		Sequence: req.Sequence,
		Count:    uint32(seq) - req.Sequence + 1,
		Status:   miax.SesMStatusAccepted,
	}
	errs.CheckE(s.bconn.WriteMessageSimple(&res))
	errs.CheckE(s.sendAll(int(req.Sequence), seq+1))
	s.waitForMcast(seq)
	res2 := miax.MessageSesMFinished{
		Sequence: req.Sequence,
	}
	errs.CheckE(s.bconn.WriteMessageSimple(&res2))
	log.Println("sesm finished")
}
func (s *SesMServerConn) login() (err error) {
	defer errs.PassE(&err)
	m, err := s.bconn.ReadMessage()
	errs.CheckE(err)
	_, ok := m.(*miax.MessageLogin)
	errs.Check(ok)
	res := miax.MessageLoginResponse{
		Status: miax.LoginAccepted,
	}
	errs.CheckE(s.bconn.WriteMessageSimple(&res))
	log.Printf("login done")
	return
}
func (s *SesMServerConn) sendImageAvail(cancel <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case _, _ = <-cancel:
			log.Printf("image avail cancelled")
			return
		case <-ticker.C:
			seq := s.src.CurrentSequence() - s.imageLag
			if seq > 0 {
				log.Printf("image avail %d", seq)
				sia := miax.MessageSesMImageAvail{
					Sequence: uint32(seq),
				}
				errs.CheckE(s.bconn.WriteMessageSimple(&sia))
			}
		}
	}
}
func (s *SesMServerConn) sendAll(start, end int) (err error) {
	defer errs.PassE(&err)
	log.Printf("sesm send %d .. %d", start, end)
	for i := start; i < end; i++ {
		m := s.src.GetMessage(i)
		errs.CheckE(s.bconn.WriteMessageSimple(m))
	}
	log.Printf("sesm send %d .. %d done", start, end)
	return
}
func (s *SesMServerConn) waitForMcast(startSeq int) {
	waitSeq := startSeq + s.mcastDuringSesM
	log.Printf("wait for mcast seq %d, current %d", waitSeq, s.src.CurrentSequence())
	bmsc := s.src.NewClient()
	defer bmsc.Close()
	ch := bmsc.Chan()
	for seq := s.src.CurrentSequence(); seq < waitSeq; seq = <-ch {
	}
}

type miaxMcastServer struct {
	laddr string
	raddr string
	src   *miaxMessageSource

	cancel chan struct{}
	bmsc   *miaxMessageSourceClient
	pw     miax.PacketWriter
	conn   net.Conn
}

func newMiaxMcastServer(laddr, raddr string, src *miaxMessageSource) *miaxMcastServer {
	return &miaxMcastServer{
		laddr:  laddr,
		raddr:  raddr,
		src:    src,
		cancel: make(chan struct{}),
	}
}
func (s *miaxMcastServer) start() (err error) {
	defer errs.PassE(&err)
	laddr, err := net.ResolveUDPAddr("udp", s.laddr)
	errs.CheckE(err)
	raddr, err := net.ResolveUDPAddr("udp", s.raddr)
	errs.CheckE(err)
	s.conn, err = net.DialUDP("udp", laddr, raddr)
	errs.CheckE(err)
	bconn := miax.NewConn(s.conn)
	s.pw = bconn.GetPacketWriterUnsync()
	s.bmsc = s.src.NewClient()
	go s.run()
	return
}
func (s *miaxMcastServer) run() {
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

type miaxMessageSource struct {
	curSeq int64
	cancel chan struct{}
	bchan  bchan.Bchan
	mps    int
}

func NewMiaxMessageSource() *miaxMessageSource {
	return &miaxMessageSource{
		cancel: make(chan struct{}),
		bchan:  bchan.NewBchan(),
		mps:    1,
		curSeq: 1000000,
	}
}
func (bms *miaxMessageSource) Run() {
	ticker := time.NewTicker(time.Duration(1000000000/bms.mps) * time.Nanosecond)
	defer ticker.Stop()
	defer bms.bchan.Close()
	for {
		select {
		case _, _ = <-bms.cancel:
			return
		case <-ticker.C:
			bms.produceOne()
		}
	}
}
func (bms *miaxMessageSource) RunInteractive() {
	for {
		fmt.Printf("enter source seq: ")
		var seq int
		_, err := fmt.Scan(&seq)
		errs.CheckE(err)
		bms.produce(seq)
	}
}
func (bms *miaxMessageSource) publish(seq int) {
	select {
	case bms.bchan.ProducerChan() <- seq:
		log.Printf("publish source seq %d", seq)
	default:
	}
}
func (bms *miaxMessageSource) produceOne() {
	seq := int(atomic.AddInt64(&bms.curSeq, int64(1)))
	bms.publish(seq)
}
func (bms *miaxMessageSource) produce(seq int) {
	bms.SetSequence(seq)
	bms.publish(seq)
}
func (bms *miaxMessageSource) Stop() {
	close(bms.cancel)
}
func (bms *miaxMessageSource) SetSequence(seq int) {
	atomic.StoreInt64(&bms.curSeq, int64(seq))
}
func (bms *miaxMessageSource) CurrentSequence() int {
	return int(atomic.LoadInt64(&bms.curSeq))
}
func (bms *miaxMessageSource) NewClient() *miaxMessageSourceClient {
	c := &miaxMessageSourceClient{
		bc: bms.bchan.NewConsumer(),
		ch: make(chan int),
	}
	go c.run()
	return c
}
func (bms *miaxMessageSource) GetMessage(seqNum int) miax.Message {
	m := miax.MessageAddOrder{
		TimeOffset: uint32(seqNum),
		OrderId:    uint64(seqNum),
		Price:      uint64(seqNum),
		Side:       'B',
		Quantity:   10,
	}
	return &m
}

type miaxMessageSourceClient struct {
	bc bchan.BchanConsumer
	ch chan int
}

func (c *miaxMessageSourceClient) Chan() chan int {
	return c.ch
}
func (c *miaxMessageSourceClient) run() {
	for val := range c.bc.Chan() {
		//log.Printf("forwarding value %v to chan %v", val, c.ch)
		c.ch <- val.(int)
	}
	close(c.ch)
}
func (c *miaxMessageSourceClient) Close() {
	c.bc.Close()
}
