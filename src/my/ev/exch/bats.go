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
	SetSequence(int)
	CurrentSequence() int
	GetMessage(int) bats.Message
	Run()
	RunInteractive()
	Stop()
}

type exchangeBatsRegistry struct {
	exchangeBatsN      []*exchangeBats
	batsMessageSourceN []*batsMessageSource
}

func (e *exchangeBatsRegistry) NewBatsRegistry(c Config, src *batsMessageSource, num int)  (eb *exchangeBats){
	var err error
	laddr, err := net.ResolveUDPAddr("udp", c.LocalAddr)
	errs.CheckE(err)
	raddr, err := net.ResolveUDPAddr("udp", c.RemoteAddr)
	errs.CheckE(err)

	laddr.Port += num
	raddr.Port += num
	raddr.IP[net.IPv6len - 1] += (byte)(num / 4)

	eb = &exchangeBats{
		interactive: c.Interactive,
		src:         src,
		spin: &spinServer{
			laddr: fmt.Sprintf(":%d", 16002 + num),
			src:   src,
		},
		mcast: newBatsMcastServer(laddr, raddr, src, num),
	}
	return
}
func InitBatsRegistry(c Config) (es ExchangeSimulator) {
	esr := &exchangeBatsRegistry{}
	for i := 0; i < c.ConnNumLimit; i++ {
		src := NewBatsMessageSource(i)
		esr.exchangeBatsN = append(esr.exchangeBatsN, esr.NewBatsRegistry(c, src, i))
		esr.exchangeBatsN[i].num = i;
	}
	es = esr
	return
}
func (e *exchangeBatsRegistry) Run() {
	for _,r := range e.exchangeBatsN {
		if r.interactive {
			go r.src.RunInteractive()
		} else {
			go r.src.Run()
		}
		go r.spin.run()
		errs.CheckE(r.mcast.start(r.num))
		log.Println(r.num,"started local", r.mcast.laddr.String(), "mcast", r.mcast.raddr.String())
	}
	select {}
}

func NewBatsExchangeSimulatorServer(c Config) (es ExchangeSimulator, err error) {
	errs.Check(c.Protocol == "bats")
	es = InitBatsRegistry(c)
	return
}

type exchangeBats struct {
	interactive bool
	src         MessageSource
	spin        *spinServer
	mcast       *batsMcastServer
	num         int
}

type spinServer struct {
	laddr string
	src   *batsMessageSource
}

func (s *spinServer) run() {
	l, err := net.Listen("tcp", s.laddr)
	errs.CheckE(err)
	defer l.Close()
	log.Println(s.src.num,"started tcp", s.laddr)
	for {
		conn, err := l.Accept()
		errs.CheckE(err)
		log.Printf("accepted %s -> %s \n", conn.RemoteAddr(), conn.LocalAddr())
		c := NewSpinServerConn(conn, s.src)
		go c.run()
	}
}

type spinServerConn struct {
	conn            net.Conn
	bconn           bats.Conn
	src             *batsMessageSource
	imageLag        int
	mcastDuringSpin int
}

func NewSpinServerConn(conn net.Conn, src *batsMessageSource) *spinServerConn {
	return &spinServerConn{
		conn:            conn,
		bconn:           bats.NewConn(conn),
		src:             src,
		imageLag:        10,
		mcastDuringSpin: 10,
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
	s.waitForMcast(seq)
	res2 := bats.MessageSpinFinished{
		Sequence: req.Sequence,
	}
	errs.CheckE(s.bconn.WriteMessageSimple(&res2))
	log.Println("spin finished")
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
			log.Printf("image avail cancelled")
			return
		case <-ticker.C:
			seq := s.src.CurrentSequence() - s.imageLag
			if seq > 0 {
				log.Printf("image avail %d", seq)
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
	log.Printf("spin send %d .. %d", start, end)
	for i := start; i < end; i++ {
		m := s.src.GetMessage(i)
		errs.CheckE(s.bconn.WriteMessageSimple(m))
	}
	log.Printf("spin send %d .. %d done", start, end)
	return
}
func (s *spinServerConn) waitForMcast(startSeq int) {
	waitSeq := startSeq + s.mcastDuringSpin
	log.Printf("wait for mcast seq %d, current %d", waitSeq, s.src.CurrentSequence())
	bmsc := s.src.NewClient()
	defer bmsc.Close()
	ch := bmsc.Chan()
	for seq := s.src.CurrentSequence(); seq < waitSeq; seq = <-ch {
	}
}

type batsMcastServer struct {
	laddr *net.UDPAddr
	raddr *net.UDPAddr
	src   *batsMessageSource

	cancel chan struct{}
	bmsc   *batsMessageSourceClient
	pw     bats.PacketWriter
	conn   net.Conn
	num    int
}

func newBatsMcastServer(laddr, raddr *net.UDPAddr, src *batsMessageSource, i int) *batsMcastServer {
	return &batsMcastServer{
		laddr :  laddr,
		raddr :  raddr,
		src:    src,
		cancel: make(chan struct{}),
		num:    i,
	}
}
func (s *batsMcastServer) start(num int) (err error) {
	s.conn, err = net.DialUDP("udp", s.laddr, s.raddr)
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

	log.Printf("%d ready. source chan %v", s.num, ch)
	for {
		select {
		case _, _ = <-s.cancel:
			log.Printf("%d cancelled", s.num)
			return
		case seq := <-ch:
			log.Printf("%d mcast seq %d", s.num, seq)
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
	num    int
}

func NewBatsMessageSource(i int) *batsMessageSource {
	return &batsMessageSource{
		cancel: make(chan struct{}),
		bchan:  bchan.NewBchan(),
		mps:    1,
		curSeq: 1000000,
		num:    i,
	}
}
func (bms *batsMessageSource) Run() {
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
func (bms *batsMessageSource) RunInteractive() {
	for {
		fmt.Printf("enter source seq: ")
		var seq int
		_, err := fmt.Scan(&seq)
		errs.CheckE(err)
		bms.produce(seq)
	}
}
func (bms *batsMessageSource) publish(seq int) {
	select {
	case bms.bchan.ProducerChan() <- seq:
		log.Printf("%d publish source seq %d", bms.num, seq)
	default:
	}
}
func (bms *batsMessageSource) produceOne() {
	seq := int(atomic.AddInt64(&bms.curSeq, int64(1)))
	bms.publish(seq)
}
func (bms *batsMessageSource) produce(seq int) {
	bms.SetSequence(seq)
	bms.publish(seq)
}
func (bms *batsMessageSource) Stop() {
	close(bms.cancel)
}
func (bms *batsMessageSource) SetSequence(seq int) {
	atomic.StoreInt64(&bms.curSeq, int64(seq))
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
