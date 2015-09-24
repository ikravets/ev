// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package bats

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"math"
	"sync"

	"github.com/ikravets/errs"
)

type MessageType byte

type Message interface {
	getCommon() *MessageCommon
	//getHeader() MessageHeader
	Type() MessageType
}

const (
	TypeLogin          MessageType = 0x01
	TypeLoginResponse  MessageType = 0x02
	TypeGapRequest     MessageType = 0x03
	TypeGapResponse    MessageType = 0x04
	TypeSpinImageAvail MessageType = 0x80
	TypeSpinRequest    MessageType = 0x81
	TypeSpinResponse   MessageType = 0x82
	TypeSpinFinished   MessageType = 0x83
	TypeAddOrder       MessageType = 0x21
)

var MessageLength = [256]int{
	TypeLogin:          22,
	TypeLoginResponse:  3,
	TypeGapRequest:     9,
	TypeGapResponse:    10,
	TypeSpinImageAvail: 6,
	TypeSpinRequest:    6,
	TypeSpinResponse:   11,
	TypeSpinFinished:   6,
	TypeAddOrder:       34,
}

var MessageFactory = [256]func() Message{
	TypeLogin:          func() Message { return &MessageLogin{} },
	TypeLoginResponse:  func() Message { return &MessageLoginResponse{} },
	TypeGapRequest:     func() Message { return &MessageGapRequest{} },
	TypeGapResponse:    func() Message { return &MessageGapResponse{} },
	TypeSpinImageAvail: func() Message { return &MessageSpinImageAvail{} },
	TypeSpinRequest:    func() Message { return &MessageSpinRequest{} },
	TypeSpinResponse:   func() Message { return &MessageSpinResponse{} },
	TypeSpinFinished:   func() Message { return &MessageSpinFinished{} },
	TypeAddOrder:       func() Message { return &MessageAddOrder{} },
}

func (_ *MessageLogin) Type() MessageType          { return TypeLogin }
func (_ *MessageLoginResponse) Type() MessageType  { return TypeLoginResponse }
func (_ *MessageGapRequest) Type() MessageType     { return TypeGapRequest }
func (_ *MessageGapResponse) Type() MessageType    { return TypeGapResponse }
func (_ *MessageSpinImageAvail) Type() MessageType { return TypeSpinImageAvail }
func (_ *MessageSpinRequest) Type() MessageType    { return TypeSpinRequest }
func (_ *MessageSpinResponse) Type() MessageType   { return TypeSpinResponse }
func (_ *MessageSpinFinished) Type() MessageType   { return TypeSpinFinished }
func (_ *MessageAddOrder) Type() MessageType       { return TypeAddOrder }

type MessageHeader struct {
	Length uint8
	Type   MessageType
}

type MessageCommon struct {
	Header MessageHeader
}

func (mc *MessageCommon) getCommon() *MessageCommon {
	return mc
}
func (mc *MessageCommon) setHeader(Type MessageType) (err error) {
	defer errs.PassE(&err)
	mc.Header.Type = Type
	mc.Header.Length = uint8(MessageLength[Type])
	errs.Check(mc.Header.Length != 0)
	return
}

type MessageLogin struct {
	MessageCommon
	SessionSubId [4]byte
	Username     [4]byte
	Filler       [2]byte
	Password     [10]byte
}

const (
	LoginAccepted               = 'A'
	LoginRejectedUnauthorized   = 'N'
	LoginRejectedSessionInUse   = 'B'
	LoginRejectedInvalidSession = 'S'
)

type MessageLoginResponse struct {
	MessageCommon
	Status byte
}

type MessageGapRequest struct {
	MessageCommon
	Unit     uint8
	Sequence uint32
	Count    uint16
}

const (
	GapStatusAccepted    = 'A'
	GapStatusRange       = 'O'
	GapStatusQuotaDaily  = 'D'
	GapStatusQuotaMinute = 'M'
	GapStatusQuotaSecond = 'S'
	GapStatusCountLimit  = 'C'
	GapStatusInvalidUnit = 'I'
	GapStatusUnitUnavail = 'U'
)

type MessageGapResponse struct {
	MessageCommon
	Unit     uint8
	Sequence uint32
	Count    uint8
	Status   byte
}

type MessageSpinImageAvail struct {
	MessageCommon
	Sequence uint32
}

type MessageSpinRequest struct {
	MessageCommon
	Sequence uint32
}

const (
	SpinStatusAccepted   = 'A'
	SpinStatusRange      = 'O'
	SpinStatusInprogress = 'S'
)

type MessageSpinResponse struct {
	MessageCommon
	Sequence uint32
	Count    uint32
	Status   byte
}

type MessageSpinFinished struct {
	MessageCommon
	Sequence uint32
}

type MessageAddOrder struct {
	MessageCommon
	TimeOffset uint32
	OrderId    uint64
	Side       byte
	Quantity   uint32
	Symbol     [6]byte
	Price      uint64
	Flags      byte
}

type BsuHeader struct {
	Length   uint16
	Count    uint8
	Unit     uint8
	Sequence uint32
}

type Conn interface {
	ReadMessage() (m Message, err error)
	GetPacketWriter() PacketWriter
	GetPacketWriterUnsync() PacketWriter
	WriteMessageSimple(m Message) (err error)
}
type PacketWriter interface {
	SyncStart()
	SetSequence(int) (err error)
	SetUnit(int) (err error)
	WriteMessage(m Message) (err error)
	Flush() (err error)
	Reset()
}

type conn struct {
	rw            io.ReadWriter
	pw            PacketWriter
	wLock         sync.Locker
	messageReader io.LimitedReader
	rbuf          bytes.Buffer
	nextSeqNum    uint32
	unit          int
}

func NewConn(rw io.ReadWriter) Conn {
	wLock := &sync.Mutex{}
	return &conn{
		rw:    rw,
		wLock: wLock,
		pw:    NewPacketWriter(rw, wLock),
	}
}

func (c *conn) readBsuHeader() (err error) {
	defer errs.PassE(&err)
	errs.Check(c.messageReader.N == 0, c.messageReader.N)
	var h BsuHeader
	errs.CheckE(binary.Read(c.rw, binary.LittleEndian, &h))
	log.Printf("rcv BSU header %#v", h)
	if h.Sequence != 0 {
		errs.Check(h.Sequence == c.nextSeqNum, h.Sequence, c.nextSeqNum)
		c.nextSeqNum += uint32(h.Count)
	}
	if h.Unit == 0 {
		errs.Check(h.Sequence == 0, h.Sequence)
	} else {
		if c.unit == 0 {
			c.unit = int(h.Unit)
		} else {
			errs.Check(c.unit == int(h.Unit), c.unit, h.Unit)
		}
	}
	c.messageReader = io.LimitedReader{R: c.rw, N: int64(h.Length) - 8}
	return
}

func (c *conn) ReadMessage() (m Message, err error) {
	defer errs.PassE(&err)
	for c.messageReader.N == 0 {
		errs.CheckE(c.readBsuHeader())
	}

	c.rbuf.Reset()
	var b [2]byte
	var n int
	_, err = io.ReadFull(&c.messageReader, b[:])
	errs.CheckE(err)
	log.Printf("rcv bytes %v", b)
	n, err = c.rbuf.Write(b[:])
	errs.CheckE(err)
	errs.Check(n == len(b))
	h := MessageHeader{
		Length: uint8(b[0]),
		Type:   MessageType(b[1]),
	}
	log.Printf("rcv header %#v", h)
	io.CopyN(&c.rbuf, &c.messageReader, int64(h.Length)-2)
	f := MessageFactory[h.Type]
	errs.Check(f != nil)
	m = f()
	errs.CheckE(binary.Read(&c.rbuf, binary.LittleEndian, m))
	log.Printf("rcv %#v\n", m)
	return
}
func (c *conn) GetPacketWriter() PacketWriter {
	return NewPacketWriter(c.rw, c.wLock)
}
func (c *conn) GetPacketWriterUnsync() PacketWriter {
	return NewPacketWriter(c.rw, nil)
}
func (c *conn) WriteMessageSimple(m Message) (err error) {
	defer errs.PassE(&err)
	c.pw.SyncStart()
	errs.CheckE(c.pw.WriteMessage(m))
	errs.CheckE(c.pw.Flush())
	return
}

type packetWriter struct {
	pb   bytes.Buffer
	mb   bytes.Buffer
	bh   BsuHeader
	w    io.Writer
	lock sync.Locker
}

type nilLocker struct{}

func (_ *nilLocker) Lock()   {}
func (_ *nilLocker) Unlock() {}

func NewPacketWriter(w io.Writer, lock sync.Locker) PacketWriter {
	if lock == nil {
		lock = &nilLocker{}
	}
	return &packetWriter{w: w, lock: lock}
}
func (p *packetWriter) SyncStart() {
	p.lock.Lock()
}
func (p *packetWriter) SetSequence(seq int) (err error) {
	defer errs.PassE(&err)
	errs.Check(seq >= 0 && seq <= math.MaxUint32)
	p.bh.Sequence = uint32(seq)
	return
}
func (p *packetWriter) SetUnit(unit int) (err error) {
	defer errs.PassE(&err)
	errs.Check(unit >= 0 && unit <= math.MaxUint8)
	p.bh.Unit = uint8(unit)
	return
}
func (p *packetWriter) WriteMessage(m Message) (err error) {
	defer errs.PassE(&err)
	m.getCommon().setHeader(m.Type())
	errs.CheckE(binary.Write(&p.mb, binary.LittleEndian, m))
	p.bh.Count++
	errs.Check(p.bh.Count != 0)
	return
}
func (p *packetWriter) Flush() (err error) {
	defer errs.PassE(&err)
	length := p.mb.Len() + 8
	errs.Check(length <= math.MaxUint16, length)
	p.bh.Length = uint16(length)
	errs.CheckE(binary.Write(&p.pb, binary.LittleEndian, p.bh))
	_, err = p.mb.WriteTo(&p.pb)
	errs.CheckE(err)
	_, err = p.w.Write(p.pb.Bytes())
	errs.CheckE(err)
	p.Reset()
	return
}
func (p *packetWriter) Reset() {
	p.mb.Reset()
	p.pb.Reset()
	p.bh = BsuHeader{}
	p.lock.Unlock()
}
