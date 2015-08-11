// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package bats

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"

	"my/errs"
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
	filler       [2]byte
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
	WriteMessage(m Message) (err error)
}

type conn struct {
	rw             io.ReadWriter
	messageReader  io.LimitedReader
	buf            bytes.Buffer
	nextSeqNum     uint32
	unreadMessages uint8
	unit           int
}

func NewConn(c net.Conn) *conn {
	return &conn{
		rw: c,
	}
}

func (c conn) readBsuHeader() (err error) {
	defer errs.PassE(&err)
	errs.Check(c.unreadMessages == 0, c.unreadMessages)
	errs.Check(c.messageReader.N == 0, c.messageReader.N)
	var h BsuHeader
	errs.CheckE(binary.Read(c.rw, binary.LittleEndian, &h))
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
	c.unreadMessages = h.Count
	c.messageReader = io.LimitedReader{R: c.rw, N: int64(h.Length)}
	return
}

func (c conn) readBsuHeaderMaybe() (err error) {
	if c.unreadMessages != 0 {
		c.unreadMessages--
		return
	}
	return c.readBsuHeader()
}

func (c conn) ReadMessage() (m Message, err error) {
	defer errs.PassE(&err)
	errs.CheckE(c.readBsuHeaderMaybe())

	c.buf.Reset()
	var b [2]byte
	var n int
	_, err = io.ReadFull(&c.messageReader, b[:])
	n, err = c.buf.Write(b[:])
	errs.CheckE(err)
	errs.Check(n == len(b))
	h := MessageHeader{
		Length: uint8(b[0]),
		Type:   MessageType(b[1]),
	}
	io.CopyN(&c.buf, &c.messageReader, int64(h.Length)-2)
	f := MessageFactory[h.Type]
	errs.Check(f != nil)
	m = f()
	errs.CheckE(binary.Read(&c.buf, binary.LittleEndian, m))
	log.Printf("rcv %#v\n", m)
	return
}
func (c conn) WriteMessage(m Message) (err error) {
	defer errs.PassE(&err)
	m.getCommon().setHeader(m.Type())
	log.Printf("snd %#v\n", m)
	bh := BsuHeader{
		Length: uint16(m.getCommon().Header.Length) + 8,
		Count:  1,
	}
	errs.CheckE(binary.Write(c.rw, binary.LittleEndian, bh))
	errs.CheckE(binary.Write(c.rw, binary.LittleEndian, m))
	return
}
