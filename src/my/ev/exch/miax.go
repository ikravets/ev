// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package exch

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/ikravets/errs"
	"github.com/lunixbochs/struc"

	"my/ev/bchan"
	"my/ev/exch/miax"
)

type MiaxMessageSource interface {
	SetSequence(int)
	CurrentSequence() int
	GetMessage(uint64) miax.MachPacket
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
		mcast: newMiaxMcastServer("10.2.0.5:0", "224.0.105.1:51001", src),
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
	conn  net.Conn
	mconn miax.Conn
	src   *miaxMessageSource
}

func NewSesMServerConn(conn net.Conn, src *miaxMessageSource) *SesMServerConn {
	return &SesMServerConn{
		conn:  conn,
		mconn: miax.NewConn(conn),
		src:   src,
	}
}

func (s *SesMServerConn) run() {
	defer errs.Catch(func(ce errs.CheckerError) {
		log.Printf("caught %s\n", ce)
	})
	defer s.conn.Close()
	errs.CheckE(s.login())
	cancelSendHeartbeat := make(chan struct{})
	defer func() {
		// close channel only if not already closed
		select {
		case _, ok := <-cancelSendHeartbeat:
			if !ok {
				return
			}
		default:
		}
		close(cancelSendHeartbeat)
	}()
	go s.sendHeartbeat(cancelSendHeartbeat)

	for {
		m, err := s.mconn.ReadMessage()
		errs.CheckE(err)

		mtype := m.Type()
		switch mtype {
		case miax.TypeSesMRetransmRequest:
			rt, ok := m.(*miax.SesMRetransmRequest)
			errs.Check(ok)
			errs.CheckE(s.sendAll(rt.StartSeqNumber, rt.EndSeqNumber))
			bye := miax.SesMGoodBye{
				Reason: miax.GoodByeReasonTerminating,
			}
			errs.CheckE(s.mconn.WriteMessageSimple(&bye))
			return
		case miax.TypeSesMUnseq:
			rf, ok := m.(*miax.SesMRefreshRequest)
			errs.Check(ok)
			errs.Check(rf.RefreshType == miax.SesMRefreshToM || rf.RefreshType == miax.SesMRefreshSeriesUpdate)
			// TODO send miax system time first! (ToM 1.8, 3.2.2.2 note)
			errs.CheckE(s.mconn.WriteMachMessage(uint64(s.src.CurrentSequence())-1, s.src.generateRefreshResponse(rf.RefreshType, 5)))
			errs.CheckE(s.mconn.WriteMachMessage(uint64(s.src.CurrentSequence()), s.src.generateRefreshResponse(rf.RefreshType, 6)))
		}
	}
	log.Println("sesm finished")
}

func (s *SesMServerConn) login() (err error) {
	defer errs.PassE(&err)
	m, err := s.mconn.ReadMessage() //////// miax conn
	errs.CheckE(err)
	lr, ok := m.(*miax.SesMLoginRequest)
	errs.Check(ok)
	// проверка, что мы не хотим рефреш
	errs.Check(0 == lr.ReqSeqNum)
	res := miax.SesMLoginResponse{
		LoginStatus:   miax.LoginStatusSuccess,
		HighestSeqNum: uint64(s.src.CurrentSequence()),
	}
	errs.CheckE(s.mconn.WriteMessageSimple(&res))
	log.Printf("login done")
	return
}
func (s *SesMServerConn) sendHeartbeat(cancel <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case _, _ = <-cancel:
			log.Printf("heartbeat cancelled")
			return
		case <-ticker.C:
			seq := s.src.CurrentSequence()
			log.Printf("heartbeats %d", seq)
			sia := miax.SesMServerHeartbeat{}
			errs.CheckE(s.mconn.WriteMessageSimple(&sia))
		}
	}
}
func (s *SesMServerConn) sendAll(start, end uint64) (err error) {
	defer errs.PassE(&err)
	log.Printf("sesm start retransm %d .. %d", start, end)
	for i := start; i <= end; i++ {
		m := s.src.GetMessage(i)
		errs.CheckE(s.mconn.WriteMachPacket(m))
	}
	log.Printf("sesm retransm %d .. %d done", start, end)
	return
}

type miaxMcastServer struct {
	laddr string
	raddr string
	src   *miaxMessageSource

	cancel chan struct{}
	mmsc   *miaxMessageSourceClient
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
	//	mconn := miax.NewConn(s.conn)
	s.mmsc = s.src.NewClient()
	go s.run()
	return
}
func (s *miaxMcastServer) run() {
	var mb bytes.Buffer
	defer s.conn.Close()
	defer s.mmsc.Close()
	ch := s.mmsc.Chan()

	log.Printf("ready. source chan %v", ch)
	for {
		select {
		case _, _ = <-s.cancel:
			log.Printf("cancelled")
			return
		case seq := <-ch:
			log.Printf("mcast seq %d", seq)
			msg := s.src.GetMessage(uint64(seq))
			// FIXME this is big endian by default?
			errs.CheckE(struc.Pack(&mb, &msg))
			_, err := s.conn.Write(mb.Bytes())
			errs.CheckE(err)
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
		curSeq: 0,
	}
}
func (mms *miaxMessageSource) Run() {
	ticker := time.NewTicker(time.Duration(1000000000/mms.mps) * time.Nanosecond)
	defer ticker.Stop()
	defer mms.bchan.Close()
	for {
		select {
		case _, _ = <-mms.cancel:
			return
		case <-ticker.C:
			mms.produceOne()
		}
	}
}
func (mms *miaxMessageSource) RunInteractive() {
	for {
		fmt.Printf("enter source seq: ")
		var seq int
		_, err := fmt.Scan(&seq)
		errs.CheckE(err)
		mms.produce(seq)
	}
}
func (mms *miaxMessageSource) publish(seq int) {
	select {
	case mms.bchan.ProducerChan() <- seq:
		log.Printf("publish source seq %d", seq)
	default:
	}
}
func (mms *miaxMessageSource) produceOne() {
	seq := int(atomic.AddInt64(&mms.curSeq, int64(1)))
	mms.publish(seq)
}
func (mms *miaxMessageSource) produce(seq int) {
	mms.SetSequence(seq)
	mms.publish(seq)
}
func (mms *miaxMessageSource) Stop() {
	close(mms.cancel)
}
func (mms *miaxMessageSource) SetSequence(seq int) {
	atomic.StoreInt64(&mms.curSeq, int64(seq))
}
func (mms *miaxMessageSource) CurrentSequence() int {
	return int(atomic.LoadInt64(&mms.curSeq))
}
func (mms *miaxMessageSource) NewClient() *miaxMessageSourceClient {
	c := &miaxMessageSourceClient{
		bc: mms.bchan.NewConsumer(),
		ch: make(chan int),
	}
	go c.run()
	return c
}
func (mms *miaxMessageSource) GetMessage(seqNum uint64) miax.MachPacket { //, mtype miax.SesMMessageType
	m := &miax.MachToMWide{
		NanoTime:      uint32(seqNum),             //Nanoseconds part of the timestamp
		ProductID:     uint32(seqNum),             //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
		MBBOPrice:     uint32(seqNum % 10 * 1000), //MIAX Best price at the time stated in Timestamp and side specified in Message Type
		MBBOSize:      uint32(seqNum % 10),        //Aggregate size at MIAX Best Price at the time stated in Timestamp and side specified in Message Type
		MBBOPriority:  uint32((seqNum % 10) / 2),  //Aggregate size of Priority Customer contracts at the MIAX Best Price
		MBBOCondition: miax.ConditionRegular,
	}
	m.Type = miax.MachMessageType('A' + ('W'-'A')*(byte(seqNum%2)))
	return miax.MakeMachPacket(seqNum, m)
}

func (mms *miaxMessageSource) generateRefreshResponse(RefreshType byte, seqNum int) (m miax.MachMessage) {
	u32, u16 := uint32(seqNum), uint16(seqNum)
	switch RefreshType {
	case miax.SesMRefreshSeriesUpdate:
		m = &miax.MachSeriesUpdate{
			NanoTime:         u32,                                             //Product Add/Update Time. Time at which this product is added/updated on MIAX system today.
			ProductID:        u32,                                             //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
			UnderlyingSymbol: [11]byte{'q', 'w', 'e', 'r', 't', 'y'},          //Stock Symbol for the option
			SecuritySymbol:   [6]byte{'q', 'w', 'e', 'r', 't', 'y'},           //Option Security Symbol
			ExpirationDate:   [8]byte{'2', '0', '1', '5', '1', '2', '1', '2'}, //Expiration date of the option in YYYYMMDD format
			StrikePrice:      u32 % 10 * 1000,                                 //Explicit strike price of the option. Refer to data types for field processing notes
			CallPut:          'C',                                             //Option Type “C” = Call "P" = Put
			OpeningTime:      [8]byte{'0', '9', ':', '3', '0', ':', '0', '0'}, //Expressed in HH:MM:SS format. Eg: 09:30:00
			ClosingTime:      [8]byte{'1', '6', ':', '1', '5', ':', '0', '0'}, //Expressed in HH:MM:SS format. Eg: 16:15:00
			RestrictedOption: 'Y',                                             //“Y” = MIAX will accept position closing orders only “N” = MIAX will accept open and close positions
			LongTermOption:   'Y',                                             //“Y” = Far month expiration (as defined by MIAX rules) “N” = Near month expiration (as defined by MIAX rules)
			Active:           'A',                                             //Indicates if this symbol is tradable on MIAX in the current session:“A” = Active “I” = Inactive (not tradable)
			BBOIncrement:     'P',                                             //This is the Minimum Price Variation as agreed to by the Options industry (penny pilot program) and as published by MIAX
			AcceptIncrement:  'P',
		}
	case miax.SesMRefreshToM:
		m = &miax.MachDoubleSidedToMCompact{
			NanoTime:   u32,
			ProductID:  u32,
			BidPrice:   u16,
			BidSize:    u16,
			OfferPrice: u16,
			OfferSize:  u16,
		}
		if 0 == seqNum%2 {
			m = &miax.MachDoubleSidedToMWide{
				NanoTime:   u32,
				ProductID:  u32,
				BidPrice:   u32,
				BidSize:    u32,
				OfferPrice: u32,
				OfferSize:  u32,
			}
		}
	default:
		errs.Check(false)
	}
	m.SetType(m.GetType())
	return
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
