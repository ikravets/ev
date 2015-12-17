// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package exch

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/ikravets/errs"

	"my/ev/bchan"
	"my/ev/exch/miax"
)

type MiaxMessageSource interface {
	SetSequence(uint64)
	CurrentSequence() uint64
	GetMessage(uint64) miax.MachPacket
	Run()
	RunInteractive()
	Stop()
}

type exchangeMiaxRegistry struct {
	exchangeMiaxN []*exchangeMiax
}

const LocalPortShift = 100

func (e *exchangeMiaxRegistry) NewMiaxRegistry(c Config, msrc *miaxMessageSource, num int) (em *exchangeMiax) {
	mc := newMiaxMcastServer(c, msrc, num)
	em = &exchangeMiax{
		interactive: c.Interactive,
		src:         msrc,
		sesm: &SesMServer{
			laddr: fmt.Sprintf(":%d", mc.laddr.Port-LocalPortShift),
			src:   msrc,
		},
		mcast: mc,
	}
	return
}

func InitMiaxRegistry(c Config) (es ExchangeSimulator) {
	esr := &exchangeMiaxRegistry{}
	log.Printf("inited simulators %d\n", c.ConnNumLimit)
	for i := 0; i < c.ConnNumLimit; i++ {
		msrc := NewMiaxMessageSource(i)
		esr.exchangeMiaxN = append(esr.exchangeMiaxN, esr.NewMiaxRegistry(c, msrc, i))
		esr.exchangeMiaxN[i].num = i
	}
	es = esr
	return
}

func NewMiaxExchangeSimulatorServer(c Config) (es ExchangeSimulator, err error) {
	errs.Check(c.Protocol == "miax")
	es = InitMiaxRegistry(c)
	return
}

type exchangeMiax struct {
	interactive bool
	src         MiaxMessageSource
	sesm        *SesMServer
	mcast       *miaxMcastServer
	num         int
}

func (e *exchangeMiaxRegistry) Run() {
	for _, r := range e.exchangeMiaxN {
		if r.interactive {
			go r.src.RunInteractive()
		} else {
			go r.src.Run()
		}
		go r.sesm.run()
		errs.CheckE(r.mcast.start())
		log.Println(r.num, "started local", r.mcast.laddr.String(), "to ToM Real Time mcast", r.mcast.mcaddr.String())
	}
	select {}
}

type SesMServer struct {
	laddr string
	src   *miaxMessageSource
	num   int
}

func (s *SesMServer) run() {
	l, err := net.Listen("tcp", s.laddr)
	errs.CheckE(err)
	defer l.Close()
	log.Println(s.src.num, "started tcp", s.laddr)
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
	num   int
}

func NewSesMServerConn(conn net.Conn, src *miaxMessageSource) *SesMServerConn {
	return &SesMServerConn{
		conn:  conn,
		mconn: miax.NewConn(conn),
		src:   src,
		num:   src.num,
	}
}

func (s *SesMServerConn) run() {
	defer errs.Catch(func(ce errs.CheckerError) {
		log.Printf("caught %s\n", ce)
	})
	defer func() {
		errs.CheckE(s.mconn.WriteMessageSimple(&miax.SesMGoodBye{Reason: miax.GoodByeReasonTerminating}))
		s.conn.Close()
		log.Println("sesm finished")
	}()
	errs.CheckE(s.login())
	defer func() {
		errs.CheckE(s.mconn.WriteMessageSimple(&miax.SesMEndOfSession{}))
	}()
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
		if err == io.EOF {
			return
		}
		errs.CheckE(err)

		mtype := m.Type()
		switch mtype {
		case miax.TypeSesMRetransmRequest:
			rt, ok := m.(*miax.SesMRetransmRequest)
			errs.Check(ok)
			errs.CheckE(s.sendAll(rt.StartSeqNumber, rt.EndSeqNumber))
			return
		case miax.TypeSesMUnseq:
			rf, ok := m.(*miax.SesMRefreshRequest)
			errs.Check(ok)
			errs.Check(rf.RefreshType == miax.SesMRefreshToM || rf.RefreshType == miax.SesMRefreshSeriesUpdate)
			// send miax system time first! (ToM 1.8, 3.2.2.2 note)
			sn := uint64(s.src.CurrentSequence())
			stime := &miax.MachSystemTime{TimeStamp: uint32(sn)}
			stime.SetType(stime.GetType())
			errs.CheckE(s.mconn.WriteMachMessage(sn-2, stime))
			errs.CheckE(s.mconn.WriteMachMessage(sn-1, s.src.generateRefreshResponse(rf.RefreshType, 5)))
			errs.CheckE(s.mconn.WriteMachMessage(sn, s.src.generateRefreshResponse(rf.RefreshType, 6)))
			eor := miax.SesMEndRefreshNotif{
				RefreshType:  rf.RefreshType,
				ResponseType: 'E',
			}
			errs.CheckE(s.mconn.WriteMessageSimple(&eor))
		case miax.TypeSesMClientHeartbeat:
		default:
			return
		}
	}
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
	laddr  *net.UDPAddr
	mcaddr *net.UDPAddr
	src    *miaxMessageSource

	cancel chan struct{}
	mmsc   *miaxMessageSourceClient
	conn   net.Conn
	num    int
}

func newMiaxMcastServer(c Config, src *miaxMessageSource, i int) (mms *miaxMcastServer) {
	laddr, err := net.ResolveUDPAddr("udp", c.LocalAddr)
	errs.CheckE(err)
	mcaddr, err := net.ResolveUDPAddr("udp", c.FeedAddr)
	errs.CheckE(err)
	laddr.Port += i + LocalPortShift
	mcaddr.Port += i
	mcaddr.IP[net.IPv6len-1] += (byte)(i)
	mms = &miaxMcastServer{
		laddr:  laddr,
		mcaddr: mcaddr,
		src:    src,
		cancel: make(chan struct{}),
		num:    i,
	}
	return
}

func (s *miaxMcastServer) start() (err error) {
	defer errs.PassE(&err)
	s.conn, err = net.DialUDP("udp", s.laddr, s.mcaddr)
	errs.CheckE(err)
	//	mconn := miax.NewConn(s.conn)
	s.mmsc = s.src.NewClient()
	go s.run()
	return
}
func (s *miaxMcastServer) run() {
	defer s.conn.Close()
	defer s.mmsc.Close()
	ch := s.mmsc.Chan()

	log.Printf("%d ready. source chan %v", s.num, ch)
	for {
		select {
		case _, _ = <-s.cancel:
			log.Printf("%d cancelled", s.num)
			return
		case seq := <-ch:
			log.Printf("%d mcast seq %d", s.num, seq)
			msg := s.src.GetMessage(uint64(seq))
			errs.CheckE(msg.Write(s.conn))
		}
	}
}

type miaxMessageSource struct {
	curSeq uint64
	cancel chan struct{}
	bchan  bchan.Bchan
	mps    int
	num    int
}

func NewMiaxMessageSource(i int) *miaxMessageSource {
	return &miaxMessageSource{
		cancel: make(chan struct{}),
		bchan:  bchan.NewBchan(),
		mps:    1,
		curSeq: 0,
		num:    i,
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
		var seq uint64
		_, err := fmt.Scan(&seq)
		errs.CheckE(err)
		mms.produce(seq)
	}
}
func (mms *miaxMessageSource) publish(seq uint64) {
	select {
	case mms.bchan.ProducerChan() <- seq:
		log.Printf("publish source seq %d", seq)
	default:
	}
}
func (mms *miaxMessageSource) produceOne() {
	seq := atomic.AddUint64(&mms.curSeq, uint64(1))
	mms.publish(seq)
}
func (mms *miaxMessageSource) produce(seq uint64) {
	mms.SetSequence(seq)
	mms.publish(seq)
}
func (mms *miaxMessageSource) Stop() {
	close(mms.cancel)
}
func (mms *miaxMessageSource) SetSequence(seq uint64) {
	atomic.StoreUint64(&mms.curSeq, seq)
}
func (mms *miaxMessageSource) CurrentSequence() uint64 {
	return atomic.LoadUint64(&mms.curSeq)
}
func (mms *miaxMessageSource) NewClient() *miaxMessageSourceClient {
	c := &miaxMessageSourceClient{
		bc: mms.bchan.NewConsumer(),
		ch: make(chan uint64),
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
	ch chan uint64
}

func (c *miaxMessageSourceClient) Chan() chan uint64 {
	return c.ch
}
func (c *miaxMessageSourceClient) run() {
	for val := range c.bc.Chan() {
		//log.Printf("forwarding value %v to chan %v", val, c.ch)
		c.ch <- val.(uint64)
	}
	close(c.ch)
}
func (c *miaxMessageSourceClient) Close() {
	c.bc.Close()
}
