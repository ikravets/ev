// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package miax

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"

	"github.com/ikravets/errs"
)

type Conn interface {
	ReadMessage() (m SesMMessage, err error)
	WriteMessageSimple(m SesMMessage) (err error)
	WriteMachMessage(sn uint64, m MachMessage) (err error)
	WriteMachPacket(p MachPacket) (err error)
}

type conn struct {
	rw            io.ReadWriter
	messageReader io.LimitedReader
}

func NewConn(rw io.ReadWriter) Conn {
	return &conn{rw: rw}
}

func (c *conn) readSize() (err error) {
	defer errs.PassE(&err)
	errs.Check(c.messageReader.N == 0, c.messageReader.N)
	var s uint16
	errs.CheckE(binary.Read(c.rw, binary.LittleEndian, &s))
	log.Printf("rcv SesM size %d\n", s)
	c.messageReader = io.LimitedReader{R: c.rw, N: int64(s)}
	return
}

func (c *conn) ReadMessage() (m SesMMessage, err error) {
	defer errs.PassE(&err)
	for c.messageReader.N == 0 {
		errs.CheckE(c.readSize())
	}

	var t [1]byte
	h := SesMHeader{Length: uint16(c.messageReader.N)}
	_, err = io.ReadFull(&c.messageReader, t[:])
	errs.CheckE(err)
	log.Printf("rcv SesM type %c\n", t[0])
	h.Type = SesMMessageType(t[0])
	log.Printf("rcv header %#v\n", h)
	f := SesMMessageFactory[h.Type]
	errs.Check(f != nil)
	m = f()
	errs.CheckE(binary.Read(&c.messageReader, binary.LittleEndian, m))
	log.Printf("rcv %#v\n", m)
	return
}
func writeSesMHeader(w io.Writer, m SesMMessage) error {
	return binary.Write(w, binary.LittleEndian, SesMHeader{Length: m.Size() + 1, Type: m.Type()})
}
func (c *conn) WriteMessageSimple(m SesMMessage) (err error) {
	defer errs.PassE(&err)
	var b bytes.Buffer
	errs.CheckE(writeSesMHeader(&b, m))
	errs.CheckE(binary.Write(&b, binary.LittleEndian, m))
	_, err = c.rw.Write(b.Bytes())
	return
}

func (c *conn) WriteMachPacket(p MachPacket) (err error) {
	defer errs.PassE(&err)
	var b, res bytes.Buffer
	errs.CheckE(binary.Write(&b, binary.LittleEndian, p))

	sm := &SesMRetransmResponse{ApplicationMessage: b.Bytes()}
	sm.Sequence = p.h.Sequence
	writeSesMHeader(&res, sm)
	errs.CheckE(binary.Write(&res, binary.LittleEndian, sm.Sequence))
	errs.CheckE(binary.Write(&res, binary.LittleEndian, sm.ApplicationMessage))
	_, err = c.rw.Write(res.Bytes())
	return
}

func (c *conn) WriteMachMessage(sn uint64, m MachMessage) (err error) {
	defer errs.PassE(&err)
	var b, res bytes.Buffer
	errs.CheckE(binary.Write(&b, binary.LittleEndian, m))

	um := &SesMRefreshResponse{
		ResponseType:       'R',
		SequenceNumber:     sn,
		ApplicationMessage: b.Bytes(),
	}
	writeSesMHeader(&res, um)
	errs.CheckE(binary.Write(&res, binary.LittleEndian, um.ResponseType))
	errs.CheckE(binary.Write(&res, binary.LittleEndian, um.SequenceNumber))
	errs.CheckE(binary.Write(&res, binary.LittleEndian, um.ApplicationMessage))
	_, err = c.rw.Write(res.Bytes())
	return
}

const (
	TypeSesMMessageCommon   SesMMessageType = 0
	TypeSesMSeq             SesMMessageType = 'S'
	TypeSesMUnseq           SesMMessageType = 'U'
	TypeSesMLoginRequest    SesMMessageType = 'L'
	TypeSesMLoginResponse   SesMMessageType = 'R'
	TypeSesMSynchrComplete  SesMMessageType = 'C'
	TypeSesMRetransmRequest SesMMessageType = 'A'
	TypeSesMGoodBye         SesMMessageType = 'G'
	TypeSesMEndOfSession    SesMMessageType = 'E'
	TypeSesMServerHeartbeat SesMMessageType = '0'
	TypeSesMClientHeartbeat SesMMessageType = '1'
)

var SesMMessageFactory = [256]func() SesMMessage{
	TypeSesMMessageCommon:   func() SesMMessage { return &SesMMessageCommon{} },
	TypeSesMSeq:             func() SesMMessage { return &SesMSeq{} },
	TypeSesMUnseq:           func() SesMMessage { return &SesMRefreshRequest{} },
	TypeSesMLoginRequest:    func() SesMMessage { return &SesMLoginRequest{} },
	TypeSesMLoginResponse:   func() SesMMessage { return &SesMLoginResponse{} },
	TypeSesMSynchrComplete:  func() SesMMessage { return &SesMSynchrComplete{} },
	TypeSesMRetransmRequest: func() SesMMessage { return &SesMRetransmRequest{} },
	TypeSesMGoodBye:         func() SesMMessage { return &SesMGoodBye{} },
	TypeSesMEndOfSession:    func() SesMMessage { return &SesMEndOfSession{} },
	TypeSesMServerHeartbeat: func() SesMMessage { return &SesMServerHeartbeat{} },
	TypeSesMClientHeartbeat: func() SesMMessage { return &SesMClientHeartbeat{} },
}

func (_ *SesMMessageCommon) Type() SesMMessageType   { return TypeSesMMessageCommon }
func (_ *SesMSeq) Type() SesMMessageType             { return TypeSesMSeq }
func (_ *SesMUnseq) Type() SesMMessageType           { return TypeSesMUnseq }
func (_ *SesMLoginRequest) Type() SesMMessageType    { return TypeSesMLoginRequest }
func (_ *SesMLoginResponse) Type() SesMMessageType   { return TypeSesMLoginResponse }
func (_ *SesMSynchrComplete) Type() SesMMessageType  { return TypeSesMSynchrComplete }
func (_ *SesMRetransmRequest) Type() SesMMessageType { return TypeSesMRetransmRequest }
func (_ *SesMGoodBye) Type() SesMMessageType         { return TypeSesMGoodBye }
func (_ *SesMEndOfSession) Type() SesMMessageType    { return TypeSesMEndOfSession }
func (_ *SesMServerHeartbeat) Type() SesMMessageType { return TypeSesMServerHeartbeat }
func (_ *SesMClientHeartbeat) Type() SesMMessageType { return TypeSesMClientHeartbeat }

func (m *SesMMessageCommon) Size() uint16    { return 0 }
func (m *SesMSeq) Size() uint16              { return 0 }
func (m *SesMUnseq) Size() uint16            { return 0 }
func (m *SesMLoginRequest) Size() uint16     { return 35 }
func (m *SesMLoginResponse) Size() uint16    { return 10 }
func (m *SesMSynchrComplete) Size() uint16   { return 0 }
func (m *SesMRetransmRequest) Size() uint16  { return 16 }
func (m *SesMGoodBye) Size() uint16          { return 1 }
func (m *SesMEndOfSession) Size() uint16     { return 0 }
func (m *SesMServerHeartbeat) Size() uint16  { return 0 }
func (m *SesMClientHeartbeat) Size() uint16  { return 0 }
func (m *SesMRefreshResponse) Size() uint16  { return uint16(9 + len(m.ApplicationMessage)) }
func (m *SesMEndRefreshNotif) Size() uint16  { return 2 }
func (m *SesMRetransmResponse) Size() uint16 { return uint16(8 + len(m.ApplicationMessage)) }

type SesMPacket struct {
	h SesMHeader
	m SesMMessage
}

type SesMMessage interface {
	Type() SesMMessageType
	Size() uint16
}

type SesMMessageType byte

type SesMHeader struct {
	Length uint16
	Type   SesMMessageType
}

type SesMMessageCommon struct{}

type SesMSeq struct {
	SesMMessageCommon
	Sequence uint64
}

type SesMUnseq struct {
	SesMMessageCommon
}

// Messages

type SesMLoginRequest struct {
	SesMMessageCommon         // Type L
	SesMVersion       [5]byte // 1.1 (right padded with spaces)
	Username          [5]byte //Username issued by MIAX during initial setup
	ComputerID        [8]byte //ID issued by MIAX during initial setup
	ApplProtocol      [8]byte //Eg: MEI1.0 (right padded with spaces)
	ReqSession        uint8   //Specifies the session the client would like to log into, or zero to log into the currently active session.
	ReqSeqNum         uint64  //Specifies client requested sequence number
	// - next sequence number the client wants to receive upon connection, or
	// - 0 to start receiving only new messages without any replay of old messages
}

type SesMLoginResponse struct {
	SesMMessageCommon        // Type R
	LoginStatus       byte   // “ “ – Login successful
	SessionID         uint8  // The session ID of the session that is now logged into.
	HighestSeqNum     uint64 // The highest sequence number that the server currently has for the client.
}

const (
	LoginStatusSuccess       = ' ' // Login successful
	LoginStatusRejected      = 'X' // Rejected: Invalid Username/Computer ID combination
	LoginStatusNotAvail      = 'S' // Requested session is not available
	LoginStatusInvalidSeqNum = 'N' // Invalid start sequence number requested
	LoginStatusSessionIncomp = 'I' // Incompatible Session protocol version
	LoginStatusApplIncomp    = 'A' // Incompatible Application protocol version
	LoginStatusLogged        = 'L' // Request rejected because client already logged in
)

type SesMSynchrComplete struct {
	SesMMessageCommon // Type C
}

type SesMRetransmRequest struct {
	SesMMessageCommon        // Type A
	StartSeqNumber    uint64 //Sequence number of the first packet to be retransmitted
	EndSeqNumber      uint64 //Sequence number of the last packet to be retransmitted
}

type SesMLogoutRequest struct {
	SesMMessageCommon // Type X
	Reason            byte
	Text              []byte // Free form human readable text to provide more details beyond the reasons mentioned above.
}

const (
	LogoutReasonDone        = ' ' // “ “ – Graceful Logout (Done for now)
	LogoutReasonBadPacket   = 'B' // “B“ – Bad SesM Packet
	LogoutReasonTimedOut    = 'L' // “L” – Timed out waiting for Login Packet
	LogoutReasonTerminating = 'A' // “A” – Application terminating connection
)

type SesMGoodBye struct {
	SesMMessageCommon // Type G
	Reason            byte
	//	Text              []byte // Free form human readable text to provide more details beyond the reasons mentioned above.
}

const (
	GoodByeReasonBadPacket   = 'B' // “B“ – Bad SesM Packet
	GoodByeReasonTimedOut    = 'L' // “L” – Timed out waiting for Login Packet
	GoodByeReasonTerminating = 'A' // “A” – Application terminating connection
)

type SesMEndOfSession struct {
	SesMMessageCommon // Type E
}

type SesMServerHeartbeat struct {
	SesMMessageCommon // Type 0
}

type SesMClientHeartbeat struct {
	SesMMessageCommon // Type 1
}

type SesMTestPacket struct {
	SesMMessageCommon        // Type T
	Text              []byte // Free form human readable text to provide more details beyond the reasons mentioned above.
}

type SesMRefreshRequest struct {
	SesMUnseq
	RequestType byte //“R” – Refresh
	RefreshType byte
}

const (
	SesMRefreshSeriesUpdate  = 'P' //Series Update Refresh
	SesMRefreshToM           = 'Q' //Top of Market Refresh
	SesMRefreshTradingStatus = 'U' //Underlying Trading Status
	SesMRefreshSystemState   = 'S' //System State Refresh
)

type SesMRefreshResponse struct {
	SesMUnseq
	ResponseType       byte //“R” – TOM Refresh
	SequenceNumber     uint64
	ApplicationMessage []byte //Based on the message type requested.
}

type SesMEndRefreshNotif struct {
	SesMUnseq
	ResponseType byte //“E” – End of Request
	RefreshType  byte //from Refresh Request
}

type SesMRetransmResponse struct {
	SesMSeq
	ApplicationMessage []byte
}

type MachMessageType byte

const (
	TypeMachMessageCommon         MachMessageType = 0
	TypeMachSystemTime            MachMessageType = '1'
	TypeMachToMWide               MachMessageType = 'W'
	TypeMachSeriesUpdate          MachMessageType = 'P'
	TypeMachDoubleSidedToMCompact MachMessageType = 'd'
	TypeMachDoubleSidedToMWide    MachMessageType = 'D'
)

func (_ *MachMessageCommon) GetType() MachMessageType         { return TypeMachMessageCommon }
func (_ *MachSystemTime) GetType() MachMessageType            { return TypeMachSystemTime }
func (_ *MachToMWide) GetType() MachMessageType               { return TypeMachToMWide }
func (_ *MachSeriesUpdate) GetType() MachMessageType          { return TypeMachSeriesUpdate }
func (_ *MachDoubleSidedToMCompact) GetType() MachMessageType { return TypeMachDoubleSidedToMCompact }
func (_ *MachDoubleSidedToMWide) GetType() MachMessageType    { return TypeMachDoubleSidedToMWide }

func (m *MachMessageCommon) Size() uint16         { return 0 }
func (m *MachSystemTime) Size() uint16            { return 5 }
func (m *MachToMWide) Size() uint16               { return 22 }
func (m *MachSeriesUpdate) Size() uint16          { return 73 }
func (m *MachDoubleSidedToMCompact) Size() uint16 { return 23 }
func (m *MachDoubleSidedToMWide) Size() uint16    { return 35 }

type MachPacket struct {
	h MachMessageHeader
	m MachMessage
}

func (p MachPacket) Write(w io.Writer) (err error) {
	defer errs.PassE(&err)
	var b bytes.Buffer
	errs.CheckE(binary.Write(&b, binary.LittleEndian, p.h))
	errs.CheckE(binary.Write(&b, binary.LittleEndian, p.m))
	_, err = w.Write(b.Bytes())
	errs.CheckE(err)
	return
}

type MachMessage interface {
	GetType() MachMessageType
	SetType(MachMessageType)
	Size() uint16
}

func MakeMachPacket(sn uint64, m MachMessage) (p MachPacket) {
	p.h = MachMessageHeader{
		Sequence:   sn,
		PackLength: 12 + m.Size(),
		PackType:   TypeMachAppData,
		SessionNum: 0,
	}
	p.m = m
	return
}

// retransmission and multicast services have this attached, refresh service DOES NOT attach this
type MachMessageHeader struct {
	Sequence   uint64
	PackLength uint16
	PackType   uint8
	SessionNum uint8
}

const (
	TypeMachHeartbeat    = 0x00
	TypeMachStartSession = 0x01
	TypeMachEndSession   = 0x02
	TypeMachAppData      = 0x03
)

type MachMessageCommon struct {
	Type MachMessageType
}

func (m *MachMessageCommon) SetType(t MachMessageType) {
	m.Type = t
}

type MachSystemTime struct {
	MachMessageCommon        //“1”
	TimeStamp         uint32 //Seconds part of the time
}

type MachSeriesUpdate struct {
	MachMessageCommon          //“P”
	NanoTime          uint32   //Product Add/Update Time. Time at which this product is added/updated on MIAX system today.
	ProductID         uint32   //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	UnderlyingSymbol  [11]byte //Stock Symbol for the option
	SecuritySymbol    [6]byte  //Option Security Symbol
	ExpirationDate    [8]byte  //Expiration date of the option in YYYYMMDD format
	StrikePrice       uint32   //Explicit strike price of the option. Refer to data types for field processing notes
	CallPut           byte     //Option Type “C” = Call "P" = Put
	OpeningTime       [8]byte  //Expressed in HH:MM:SS format. Eg: 09:30:00
	ClosingTime       [8]byte  //Expressed in HH:MM:SS format. Eg: 16:15:00
	RestrictedOption  byte     //“Y” = MIAX will accept position closing orders only “N” = MIAX will accept open and close positions
	LongTermOption    byte     //“Y” = Far month expiration (as defined by MIAX rules) “N” = Near month expiration (as defined by MIAX rules)
	Active            byte     //Indicates if this symbol is tradable on MIAX in the current session:“A” = Active “I” = Inactive (not tradable) on MIAX
	BBOIncrement      byte     //This is the Minimum Price Variation as agreed to by the Options industry (penny pilot program) and as published by MIAX
	AcceptIncrement   byte     //This is the Minimum Price Variation for Quote/Order acceptance as per MIAX rules
	//|---Price <= $3---|-- Price > $3
	//|-----------------|---------------
	//|“P” Penny (0.01)-|- Penny (0.01)
	//|“N” Penny (0.01)-|- Nickel (0.05)
	//|“D” Nickel (0.05)|- Dime (0.10)

}

type MachSystemState struct {
	MachMessageCommon //“S”
	NanoTime          uint32
	ToMVersion        [8]byte //Eg: ToM01.01
	SessionID         uint32
	SystemStatus      byte
}

const (
	SystemStartHours      = 'S' //Start of System hours
	SystemEndHours        = 'C' //End of System hours
	SystemStarTestSession = '1' //Start of Test Session (sent before tests).
	SystemEndTestSession  = '2' //End of Test Session.
)

type MachToMCompact struct {
	MachMessageCommon        //“B” = MIAX Top of Market on Bid side, “O” = MIAX Top of Market on Offer side
	NanoTime          uint32 //Nanoseconds part of the timestamp
	ProductID         uint32 //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	MBBOPrice         uint16 //MIAX Best price at the time stated in Timestamp and side specified in Message Type
	MBBOSize          uint16 //Aggregate size at MIAX Best Price at the time stated in Timestamp and side specified in Message Type
	MBBOPriority      uint16 //Aggregate size of Priority Customer contracts at the MIAX Best Price
	MBBOCondition     byte
}

const (
	ConditionRegular      = 'A' //A Regular (Eligible for Automatic Execution)
	ConditionPublic       = 'B' //Quote contains Public Customer
	ConditionQuoteNotFirm = 'C' //Quote is not firm on this side
	ConditionReserved     = 'R' //Reserved for future use
	ConditionTradingHalt  = 'T' //Trading Halt
)

type MachToMWide struct {
	MachMessageCommon        //“W” = MIAX Top of Market on Bid side, “A” = MIAX Top of Market on Offer side
	NanoTime          uint32 //Nanoseconds part of the timestamp
	ProductID         uint32 //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	MBBOPrice         uint32 //MIAX Best price at the time stated in Timestamp and side specified in Message Type
	MBBOSize          uint32 //Aggregate size at MIAX Best Price at the time stated in Timestamp and side specified in Message Type
	MBBOPriority      uint32 //Aggregate size of Priority Customer contracts at the MIAX Best Price
	MBBOCondition     byte
}

type MachDoubleSidedToMCompact struct {
	MachMessageCommon        //“d”
	NanoTime          uint32 //Nanoseconds part of the timestamp
	ProductID         uint32 //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	BidPrice          uint16 //MIAX Best Bid price at the time stated in Timestamp and side specified in Message Type
	BidSize           uint16 //Aggregate size at MIAX Best Bid Price at the time stated in Timestamp and side specified in Message Type
	BidPriority       uint16 //Aggregate size of Priority Customer contracts at the MIAX Best Bid Price
	BidCondition      byte
	OfferPrice        uint16 //MIAX Best Offer price at the time stated in Timestamp and side specified in Message Type
	OfferSize         uint16 //Aggregate size at MIAX Best Offer Price at the time stated in Timestamp and side specified in Message Type
	OfferPriority     uint16 //Aggregate size of Priority Customer contracts at the MIAX Best Offer Price
	OfferCondition    byte
}

type MachDoubleSidedToMWide struct {
	MachMessageCommon        //“D”
	NanoTime          uint32 //Nanoseconds part of the timestamp
	ProductID         uint32 //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	BidPrice          uint32 //MIAX Best Bid price at the time stated in Timestamp and side specified in Message Type
	BidSize           uint32 //Aggregate size at MIAX Best Bid Price at the time stated in Timestamp and side specified in Message Type
	BidPriority       uint32 //Aggregate size of Priority Customer contracts at the MIAX Best Bid Price
	BidCondition      byte
	OfferPrice        uint32 //MIAX Best Offer price at the time stated in Timestamp and side specified in Message Type
	OfferSize         uint32 //Aggregate size at MIAX Best Offer Price at the time stated in Timestamp and side specified in Message Type
	OfferPriority     uint32 //Aggregate size of Priority Customer contracts at the MIAX Best Offer Price
	OfferCondition    byte
}

type MachLastSale struct {
	MachMessageCommon        //“T”
	NanoTime          uint32 //Nanoseconds part of the timestamp
	ProductID         uint32 //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	TradeID           uint32 //Unique Trade ID assigned to every Trade
	CorrNumber        uint8  // Trade correction number. 0 for New trades. Greater than or equal to 0 for trades resulting from corrections/adjustments.
	RefTradeID        uint32 //0 (zero) if new trade. Trade ID of the original trade if this trade originated as a correction of the original trade.
	RefCorrNumber     uint8  //Correction Number of the trade that was just corrected/adjusted. 0 for new trades.
	TradePrice        uint32 //Price at which this product traded
	TradeSize         uint32 //Number of contracts executed in this trade
	TradeCondition    byte
}

type MachTradeCancel struct {
	MachMessageCommon        //“X”
	NanoTime          uint32 //Nanoseconds part of the timestamp
	ProductID         uint32 //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	TradeID           uint32 //Trade ID of the Canceled Trade
	CorrNumber        uint8  //Trade correction number of the trade being canceled. 0 for New trades being canceled.
	// >=0 if this is cancel of a trade that resulted from corrections/adjustments.
	TradePrice     uint32 //Trade price of the Canceled Trade
	TradeSize      uint32 //Trade volume of the Canceled Trade
	TradeCondition byte
}

const (
	TradeConditionA = 'A' //Cancel of Trade previously reported other than as the last or opening for the particular Option
	TradeConditionB = 'B' //Trade that is Late and is out of sequence
	TradeConditionC = 'C' //Cancel of the last reported Trade for the particular Option
	TradeConditionD = 'D' //Trade that is Late and is in correct sequence
	TradeConditionE = 'E' //Cancel of the first (opening) reported Trade for the particular Option
	TradeConditionF = 'F' //Trade that is late report of the opening trade and is out of sequence
	TradeConditionG = 'G' //Cancel of the only reported Trade for the particular Option
	TradeConditionH = 'H' //Trade that is late report of the opening trade and is in correct sequence
	TradeConditionJ = 'J' //Trade due to reopening of an Option in which trading has been previously halted; process as a regular transaction
	TradeConditionR = 'R' /*Trade was the execution of an order which was “stopped” at a price that did not
	constitute a Trade-Through on another market at the time of the stop. Process like a
	normal transaction except don’t update “last”*/
	TradeConditionS = 'S' //Trade was the execution of an order identified as an Intermarket Sweep Order(ISO)
	TradeConditionX = 'X' //Trade that is Trade Through Exempt. The trade should be treated like a regular sale
)

type MachUnderlyingTdStatusNotif struct {
	MachMessageCommon          //“H”
	NanoTime          uint32   //Nanoseconds part of the timestamp
	UnderlyingSymbol  [11]byte //Underlying Symbol
	TradingStatus     byte     //“H” = MIAX has halted trading for this Underlying Symbol
	//“R” = MIAX will resume trading (reopen) for this Underlying Symbol
	//“O” = MIAX will open trading for this Underlying Symbol
	EventReason          byte   //“A” = This event resulted from automatic/market driven event “M” = MIAX manually initiated this event
	EventTimeSeconds     uint32 //Seconds portion of the expected time of the event
	EventTimeNanoSeconds uint32 //Nano-seconds portion of the expected time of the event. Specifies number of nano-seconds since the EventTimeSeconds
}
