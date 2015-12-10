// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package miax

import "github.com/ikravets/errs"

type SesMMessage interface {
	getCommon() *MessageCommon
	Type() MessageType
}

type SesMMessageType byte

type SesMUnseqHeader struct {
	Length uint16
	Type   MessageType
}

type SesMUnseqCommon struct {
	Header SesMUnseqHeader
	Payload []byte
}

func (mc *SesMUnseqCommon) getCommon() *SesMUnseqCommon {
	return mc
}
func (mc *SesMUnseqCommon) setHeader(Type SesMMessageType) (err error) {
	defer errs.PassE(&err)
	mc.Header.Length = uint16(MessageLength[Type])
	mc.Header.Type = Type // 'U' or 'S'
	errs.Check(mc.Header.Length != 0)
	return
}

type SesMSeqHeader struct {
	SesMUnseqCommon
	Sequence uint64
}

type SesMSeqCommon struct {
	Header SesMSeqHeader
}

// Messages

type SesMLoginRequest struct {
	SesMUnseqCommon         // Type L
	SesMVersion     [5]byte // 1.1 (right padded with spaces)
	Username        [5]byte //Username issued by MIAX during initial setup
	ComputerID      [8]byte //ID issued by MIAX during initial setup
	ApplProtocol    [8]byte //Eg: MEI1.0 (right padded with spaces)
	ReqSession      uint8   //Specifies the session the client would like to log into, or zero to log into the currently active session.
	ReqSeqNum       uint64  //Specifies client requested sequence number
	// - next sequence number the client wants to receive upon connection, or
	// - 0 to start receiving only new messages without any replay of old messages
}

type SesMLoginResponse struct {
	SesMUnseqCommon        // Type R
	LoginStatus     byte   // “ “ – Login successful
	SessionID       uint8  // The session ID of the session that is now logged into.
	HighestSeqNum   uint64 // The highest sequence number that the server currently has for the client.
}

const (
	LoginStatusSucscss       = ' ' // Login successful
	LoginStatusRejected      = 'X' // Rejected: Invalid Username/Computer ID combination
	LoginStatusNotAvail      = 'S' // Requested session is not available
	LoginStatusInvalidSeqNum = 'N' // Invalid start sequence number requested
	LoginStatusSessionIncomp = 'I' // Incompatible Session protocol version
	LoginStatusApplIncomp    = 'A' // Incompatible Application protocol version
	LoginStatusLogged        = 'L' // Request rejected because client already logged in
)

type SesMSynchrComplete struct {
	SesMUnseqCommon // Type C
}

type SesMRetransmRequest struct {
	SesMUnseqCommon        // Type A
	StartSeqNumber  uint64 //Sequence number of the first packet to be retransmitted
	EndSeqNumber    uint64 //Sequence number of the last packet to be retransmitted
}

type SesMSynchrComplete struct {
	SesMUnseqCommon // Type X
	Reason          byte
	Text            []byte // Free form human readable text to provide more details beyond the reasons mentioned above.
}

const (
	LogoutReasonDone        = ' ' // “ “ – Graceful Logout (Done for now)
	LogoutReasonBadPacket   = 'B' // “B“ – Bad SesM Packet
	LogoutReasonTimedOut    = 'L' // “L” – Timed out waiting for Login Packet
	LogoutReasonTerminating = 'A' // “A” – Application terminating connection
)

type SesMGoodBye struct {
	SesMUnseqCommon // Type G
	Reason          byte
	Text            []byte // Free form human readable text to provide more details beyond the reasons mentioned above.
}

const (
	GoodByeReasonBadPacket   = 'B' // “B“ – Bad SesM Packet
	GoodByeReasonTimedOut    = 'L' // “L” – Timed out waiting for Login Packet
	GoodByeReasonTerminating = 'A' // “A” – Application terminating connection
)

type SesMEndOfSession struct {
	SesMUnseqCommon // Type E
}

type SesMServerHeartbeat struct {
	SesMUnseqCommon // Type 0
}

type SesMClientHeartbeat struct {
	SesMUnseqCommon // Type 1
}

type SesMTestPacket struct {
	SesMUnseqCommon        // Type T
	Text            []byte // Free form human readable text to provide more details beyond the reasons mentioned above.
}

type SesMRefreshRequest struct {
	SesMUnseqCommon
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
	SesMUnseqCommon
	ResponseType       byte //“R” – TOM Refresh
	SequenceNumber     uint64
	ApplicationMessage []byte //Based on the message type requested.
}

type SesMEndRefreshNotif struct {
	SesMUnseqCommon
	ResponseType byte //“E” – End of Request
	RefreshType  byte //from Refresh Request
}

func SesMReadMessage(r io.Reader) (m SesMMessage, err error) {
	defer errs.PassE(&err)
	var mc SesMUnseqCommon
	errs.CheckE(binary.Read(r, binary.BigEndian, &mc.Header))
	mc.Payload = make([]byte, Length)
	n, err := r.Read(mc.Payload)
	errs.CheckE(err)
	errs.Check(n == len(mc.Payload), n, len(mc.Payload))
	switch mc.Header.Type {
	case SesMLoginRequest
		m = &SesMLoginRequest{}
	case

	case

	}
//	*m.getCommon() = mc
//	errs.CheckE(m.decodePayload())
	return
}

func SesMWriteMessage(w io.Writer, m SesMUnseqMessage) (err error) {
	defer errs.PassE(&err)
//	errs.CheckE(m.encodePayload())
	var mt SesMMessageType
	switch m.(type) {
	var mt SesMMessageType
	case *SesMLoginRequest:
		mt = SesMLoginRequest
	case

	case

	case

	}
	errs.CheckE(m.getCommon().setHeader(mt))
//	errs.CheckE(binary.Write(w, binary.BigEndian, m.getHeader()))
//	n, err := w.Write(m.getCommon().Payload)
//	errs.CheckE(err)
//	errs.Check(n == len(m.getCommon().Payload))
	return
}

type MachMessageType uint8

type MachMessageHeder struct {
	Sequence   uint64
	PackLength uint16
	PackType   MachMessageType
	SessionNum uint8
}

type MachMessageCommon struct {
	Header MachMessageHeder
}

const (
	TypeMachHeartbeat    = 0x00
	TypeMachStartSession = 0x01
	TypeMachEndSession   = 0x02
	TypeMachAppData      = 0x03
)

type MachMessageType byte

type MachSystemTime struct {
	MachMessageCommon
	Type      MachMessageType //“1”
	TimeStamp uint32          //Seconds part of the time
}

type MachSeriesUpdate struct {
	MachMessageCommon
	Type             MachMessageType //“P”
	NanoTime         uint32          //Product Add/Update Time. Time at which this product is added/updated on MIAX system today.
	ProductID        uint32          //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	UnderlyingSymbol [11]byte        //Stock Symbol for the option
	SecuritySymbol   [6]byte         //Option Security Symbol
	ExpirationDate   [8]byte         //Expiration date of the option in YYYYMMDD format
	StrikePrice      uint32          //Explicit strike price of the option. Refer to data types for field processing notes
	CallPut          byte            //Option Type “C” = Call "P" = Put
	OpeningTime      [8]byte         //Expressed in HH:MM:SS format. Eg: 09:30:00
	ClosingTime      [8]byte         //Expressed in HH:MM:SS format. Eg: 16:15:00
	RestrictedOption byte            //“Y” = MIAX will accept position closing orders only “N” = MIAX will accept open and close positions
	LongTermOption   byte            //“Y” = Far month expiration (as defined by MIAX rules) “N” = Near month expiration (as defined by MIAX rules)
	Active           byte            //Indicates if this symbol is tradable on MIAX in the current session:“A” = Active “I” = Inactive (not tradable) on MIAX
	BBOIncrement     byte            //This is the Minimum Price Variation as agreed to by the Options industry (penny pilot program) and as published by MIAX
	AcceptIncrement  byte            //This is the Minimum Price Variation for Quote/Order acceptance as per MIAX rules
	//|---Price <= $3---|-- Price > $3
	//|-----------------|---------------
	//|“P” Penny (0.01)-|- Penny (0.01)
	//|“N” Penny (0.01)-|- Nickel (0.05)
	//|“D” Nickel (0.05)|- Dime (0.10)

}

type MachSystemState struct {
	MachMessageCommon
	Type         MachMessageType //“S”
	NanoTime     uint32
	ToMVersion   [8]byte //Eg: ToM01.01
	SessionID    uint32
	SystemStatus byte
}

const (
	SystemStartHours      = 'S' //Start of System hours
	SystemEndHours        = 'C' //End of System hours
	SystemStarTestSession = '1' //Start of Test Session (sent before tests).
	SystemEndTestSession  = '2' //End of Test Session.
)

type MachToMCompact struct {
	MachMessageCommon
	Type          MachMessageType //“B” = MIAX Top of Market on Bid side, “O” = MIAX Top of Market on Offer side
	NanoTime      uint32          //Nanoseconds part of the timestamp
	ProductID     uint32          //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	MBBOPrice     uint16          //MIAX Best price at the time stated in Timestamp and side specified in Message Type
	MBBOSize      uint16          //Aggregate size at MIAX Best Price at the time stated in Timestamp and side specified in Message Type
	MBBOPriority  uint16          //Aggregate size of Priority Customer contracts at the MIAX Best Price
	MBBOCondition byte
}

const (
	ConditionRegular           = 'A' //A Regular (Eligible for Automatic Execution)
	ConditionPublic            = 'B' //Quote contains Public Customer
	ConditionQuoteNotFirm      = 'C' //Quote is not firm on this side
	ConditionReserved          = 'R' //Reserved for future use
	ConditionTrading      Halt = 'T' //Trading Halt
)

type MachToMWide struct {
	MachMessageCommon
	Type          MachMessageType //“W” = MIAX Top of Market on Bid side, “A” = MIAX Top of Market on Offer side
	NanoTime      uint32          //Nanoseconds part of the timestamp
	ProductID     uint32          //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	MBBOPrice     uint32          //MIAX Best price at the time stated in Timestamp and side specified in Message Type
	MBBOSize      uint32          //Aggregate size at MIAX Best Price at the time stated in Timestamp and side specified in Message Type
	MBBOPriority  uint32          //Aggregate size of Priority Customer contracts at the MIAX Best Price
	MBBOCondition byte
}

type MachDobleSidedToMCompact struct {
	MachMessageCommon
	Type           MachMessageType //“d”
	NanoTime       uint32          //Nanoseconds part of the timestamp
	ProductID      uint32          //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	BidPrice       uint16          //MIAX Best Bid price at the time stated in Timestamp and side specified in Message Type
	BidSize        uint16          //Aggregate size at MIAX Best Bid Price at the time stated in Timestamp and side specified in Message Type
	BidPriority    uint16          //Aggregate size of Priority Customer contracts at the MIAX Best Bid Price
	BidCondition   byte
	OfferPrice     uint16 //MIAX Best Offer price at the time stated in Timestamp and side specified in Message Type
	OfferSize      uint16 //Aggregate size at MIAX Best Offer Price at the time stated in Timestamp and side specified in Message Type
	OfferPriority  uint16 //Aggregate size of Priority Customer contracts at the MIAX Best Offer Price
	OfferCondition byte
}

type MachDobleSidedToMWide struct {
	MachMessageCommon
	Type           MachMessageType //“D”
	NanoTime       uint32          //Nanoseconds part of the timestamp
	ProductID      uint32          //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	BidPrice       uint32          //MIAX Best Bid price at the time stated in Timestamp and side specified in Message Type
	BidSize        uint32          //Aggregate size at MIAX Best Bid Price at the time stated in Timestamp and side specified in Message Type
	BidPriority    uint32          //Aggregate size of Priority Customer contracts at the MIAX Best Bid Price
	BidCondition   byte
	OfferPrice     uint32 //MIAX Best Offer price at the time stated in Timestamp and side specified in Message Type
	OfferSize      uint32 //Aggregate size at MIAX Best Offer Price at the time stated in Timestamp and side specified in Message Type
	OfferPriority  uint32 //Aggregate size of Priority Customer contracts at the MIAX Best Offer Price
	OfferCondition byte
}

type MachLastSale struct {
	MachMessageCommon
	Type           MachMessageType //“T”
	NanoTime       uint32          //Nanoseconds part of the timestamp
	ProductID      uint32          //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	TradeID        uint32          //Unique Trade ID assigned to every Trade
	CorrNumber     uint8           // Trade correction number. 0 for New trades. Greater than or equal to 0 for trades resulting from corrections/adjustments.
	RefTradeID     uint32          //0 (zero) if new trade. Trade ID of the original trade if this trade originated as a correction of the original trade.
	RefCorrNumber  uint8           //Correction Number of the trade that was just corrected/adjusted. 0 for new trades.
	TradePrice     uint32          //Price at which this product traded
	TradeSize      uint32          //Number of contracts executed in this trade
	TradeCondition byte
}

type MachTradeCancel struct {
	MachMessageCommon
	Type       MachMessageType //“X”
	NanoTime   uint32          //Nanoseconds part of the timestamp
	ProductID  uint32          //MIAX Product ID mapped to a given option. It is assigned per trading session and is valid for that session.
	TradeID    uint32          //Trade ID of the Canceled Trade
	CorrNumber uint8           //Trade correction number of the trade being canceled. 0 for New trades being canceled.
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
	MachMessageCommon
	Type             MachMessageType //“H”
	NanoTime         uint32          //Nanoseconds part of the timestamp
	UnderlyingSymbol [11]byte        //Underlying Symbol
	TradingStatus    byte            //“H” = MIAX has halted trading for this Underlying Symbol
	//“R” = MIAX will resume trading (reopen) for this Underlying Symbol
	//“O” = MIAX will open trading for this Underlying Symbol
	EventReason          byte   //“A” = This event resulted from automatic/market driven event “M” = MIAX manually initiated this event
	EventTimeSeconds     uint32 //Seconds portion of the expected time of the event
	EventTimeNanoSeconds uint32 //Nano-seconds portion of the expected time of the event. Specifies number of nano-seconds since the EventTimeSeconds
}
