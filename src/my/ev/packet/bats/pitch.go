// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package bats

import (
	"encoding/binary"
	"errors"

	"github.com/google/gopacket"

	"my/errs"

	"my/ev/packet"
)

/************************************************************************/

var LayerTypePitch = gopacket.RegisterLayerType(12001, gopacket.LayerTypeMetadata{"Pitch", gopacket.DecodeFunc(decodePitch)})

func decodePitch(data []byte, p gopacket.PacketBuilder) error {
	if len(data) < 2 {
		return errors.New("message to short")
	}
	pitchMessageType := PitchMessageType(data[1])
	return pitchMessageType.Decode(data, p)
}

/************************************************************************/
type PitchMessageType uint8

func (a PitchMessageType) Decode(data []byte, p gopacket.PacketBuilder) error {
	layer := PitchMessageTypeMetadata[a].CreateLayer()
	if err := layer.DecodeFromBytes(data, p); err != nil {
		return err
	}
	p.AddLayer(layer)
	return p.NextDecoder(layer.NextLayerType())
}
func (a PitchMessageType) String() string {
	return PitchMessageTypeMetadata[a].Name
}
func (a PitchMessageType) LayerType() gopacket.LayerType {
	return PitchMessageTypeMetadata[a].LayerType
}
func (a PitchMessageType) ToInt() int {
	return int(a)
}

/************************************************************************/
const (
	PitchMessageTypeUnknown                  PitchMessageType = 0 // not in spec, catch-all
	PitchMessageTypeTime                     PitchMessageType = 0x20
	PitchMessageTypeAddOrderLong             PitchMessageType = 0x21
	PitchMessageTypeAddOrderShort            PitchMessageType = 0x22
	PitchMessageTypeOrderExecuted            PitchMessageType = 0x23
	PitchMessageTypeOrderExecutedAtPriceSize PitchMessageType = 0x24
	PitchMessageTypeReduceSizeLong           PitchMessageType = 0x25
	PitchMessageTypeReduceSizeShort          PitchMessageType = 0x26
	PitchMessageTypeModifyOrderLong          PitchMessageType = 0x27
	PitchMessageTypeModifyOrderShort         PitchMessageType = 0x28
	PitchMessageTypeDeleteOrder              PitchMessageType = 0x29
	PitchMessageTypeTradeLong                PitchMessageType = 0x2a
	PitchMessageTypeTradeShort               PitchMessageType = 0x2b
	PitchMessageTypeTradeBreak               PitchMessageType = 0x2c
	PitchMessageTypeEndOfSession             PitchMessageType = 0x2d
	PitchMessageTypeSymbolMapping            PitchMessageType = 0x2e
	PitchMessageTypeAddOrderExpanded         PitchMessageType = 0x2f
	PitchMessageTypeTradeExpanded            PitchMessageType = 0x30
	PitchMessageTypeTradingStatus            PitchMessageType = 0x31
	PitchMessageTypeAuctionUpdate            PitchMessageType = 0x95
	PitchMessageTypeAuctionSummary           PitchMessageType = 0x96
	PitchMessageTypeUnitClear                PitchMessageType = 0x97
	PitchMessageTypeRetailPriceImprovement   PitchMessageType = 0x98
)

var PitchMessageTypeNames = [256]string{
	PitchMessageTypeUnknown:                  "PitchUnknown",
	PitchMessageTypeTime:                     "PitchTime",
	PitchMessageTypeAddOrderLong:             "PitchAddOrderLong",
	PitchMessageTypeAddOrderShort:            "PitchAddOrderShort",
	PitchMessageTypeOrderExecuted:            "PitchOrderExecuted",
	PitchMessageTypeOrderExecutedAtPriceSize: "PitchOrderExecutedAtPriceSize",
	PitchMessageTypeReduceSizeLong:           "PitchReduceSizeLong",
	PitchMessageTypeReduceSizeShort:          "PitchReduceSizeShort",
	PitchMessageTypeModifyOrderLong:          "PitchModifyOrderLong",
	PitchMessageTypeModifyOrderShort:         "PitchModifyOrderShort",
	PitchMessageTypeDeleteOrder:              "PitchDeleteOrder",
	PitchMessageTypeTradeLong:                "PitchTradeLong",
	PitchMessageTypeTradeShort:               "PitchTradeShort",
	PitchMessageTypeTradeBreak:               "PitchTradeBreak",
	PitchMessageTypeEndOfSession:             "PitchEndOfSession",
	PitchMessageTypeSymbolMapping:            "PitchSymbolMapping",
	PitchMessageTypeAddOrderExpanded:         "PitchAddOrderExpanded",
	PitchMessageTypeTradeExpanded:            "PitchTradeExpanded",
	PitchMessageTypeTradingStatus:            "PitchTradingStatus",
	PitchMessageTypeAuctionUpdate:            "PitchAuctionUpdate",
	PitchMessageTypeAuctionSummary:           "PitchAuctionSummary",
	PitchMessageTypeUnitClear:                "PitchUnitClear",
	PitchMessageTypeRetailPriceImprovement:   "PitchRetailPriceImprovement",
}

var PitchMessageCreators = [256]func() PitchMessage{
	PitchMessageTypeUnknown:                  func() PitchMessage { return &PitchMessageUnknown{} },
	PitchMessageTypeTime:                     func() PitchMessage { return &PitchMessageTime{} },
	PitchMessageTypeAddOrderLong:             func() PitchMessage { return &PitchMessageAddOrder{} },
	PitchMessageTypeAddOrderShort:            func() PitchMessage { return &PitchMessageAddOrder{} },
	PitchMessageTypeOrderExecuted:            func() PitchMessage { return &PitchMessageOrderExecuted{} },
	PitchMessageTypeOrderExecutedAtPriceSize: func() PitchMessage { return &PitchMessageOrderExecutedAtPriceSize{} },
	PitchMessageTypeReduceSizeLong:           func() PitchMessage { return &PitchMessageReduceSize{} },
	PitchMessageTypeReduceSizeShort:          func() PitchMessage { return &PitchMessageReduceSize{} },
	PitchMessageTypeModifyOrderLong:          func() PitchMessage { return &PitchMessageModifyOrder{} },
	PitchMessageTypeModifyOrderShort:         func() PitchMessage { return &PitchMessageModifyOrder{} },
	PitchMessageTypeDeleteOrder:              func() PitchMessage { return &PitchMessageDeleteOrder{} },
	PitchMessageTypeTradeLong:                func() PitchMessage { return &PitchMessageTrade{} },
	PitchMessageTypeTradeShort:               func() PitchMessage { return &PitchMessageTrade{} },
	PitchMessageTypeTradeBreak:               func() PitchMessage { return &PitchMessageTradeBreak{} },
	PitchMessageTypeEndOfSession:             func() PitchMessage { return &PitchMessageEndOfSession{} },
	PitchMessageTypeSymbolMapping:            func() PitchMessage { return &PitchMessageSymbolMapping{} },
	PitchMessageTypeAddOrderExpanded:         func() PitchMessage { return &PitchMessageAddOrder{} },
	PitchMessageTypeTradeExpanded:            func() PitchMessage { return &PitchMessageTrade{} },
	PitchMessageTypeTradingStatus:            func() PitchMessage { return &PitchMessageTradingStatus{} },
	PitchMessageTypeAuctionUpdate:            func() PitchMessage { return &PitchMessageAuctionUpdate{} },
	PitchMessageTypeAuctionSummary:           func() PitchMessage { return &PitchMessageAuctionSummary{} },
	PitchMessageTypeUnitClear:                func() PitchMessage { return &PitchMessageUnitClear{} },
	PitchMessageTypeRetailPriceImprovement:   func() PitchMessage { return &PitchMessageRetailPriceImprovement{} },
}

type EnumMessageTypeMetadata struct {
	Name        string
	LayerType   gopacket.LayerType
	CreateLayer func() PitchMessage
}

var PitchMessageTypeMetadata [256]EnumMessageTypeMetadata
var LayerClassPitch gopacket.LayerClass

const PITCH_LAYERS_BASE_NUM = 12100

func init() {
	layerTypes := make([]gopacket.LayerType, 0, 256)
	for i := 0; i < 256; i++ {
		if PitchMessageTypeNames[i] == "" {
			continue
		}
		pitchMessageType := PitchMessageType(i)
		layerTypeMetadata := gopacket.LayerTypeMetadata{
			Name:    PitchMessageTypeNames[i],
			Decoder: pitchMessageType,
		}
		layerType := gopacket.RegisterLayerType(PITCH_LAYERS_BASE_NUM+i, layerTypeMetadata)
		layerTypes = append(layerTypes, layerType)
		creator := PitchMessageCreators[i]
		createLayer := func() PitchMessage {
			m := creator()
			m.Base().Type = pitchMessageType
			return m
		}
		PitchMessageTypeMetadata[i] = EnumMessageTypeMetadata{
			Name:        PitchMessageTypeNames[i],
			LayerType:   layerType,
			CreateLayer: createLayer,
		}
	}
	for i := 0; i < 256; i++ {
		if PitchMessageTypeMetadata[i].Name == "" {
			// unknown message type
			PitchMessageTypeMetadata[i] = PitchMessageTypeMetadata[PitchMessageTypeUnknown]
		}
	}
	LayerClassPitch = gopacket.NewLayerClass(layerTypes)
}

/************************************************************************/
type PitchMessage interface {
	packet.ExchangeMessage
	gopacket.DecodingLayer
	//embed gopacket.Layer by "inlining"
	//workaround for https://github.com/golang/go/issues/6977
	LayerType() gopacket.LayerType
	LayerContents() []byte

	Base() *PitchMessageCommon
}

type PitchMessageCommon struct {
	Contents   []byte
	Length     uint8
	Type       PitchMessageType
	TimeOffset uint32
}

func (m *PitchMessageCommon) CanDecode() gopacket.LayerClass {
	return m.LayerType()
}
func (m *PitchMessageCommon) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}
func (m *PitchMessageCommon) LayerContents() []byte {
	return m.Contents
}
func (m *PitchMessageCommon) LayerPayload() []byte {
	return nil
}
func (m *PitchMessageCommon) LayerType() gopacket.LayerType {
	return m.Type.LayerType()
}

func (m *PitchMessageCommon) Base() *PitchMessageCommon {
	return m
}
func (m *PitchMessageCommon) Nanoseconds() int {
	return int(m.TimeOffset)
}

func decodePitchMessage(data []byte) PitchMessageCommon {
	if len(data) < 2 {
		panic("message to short")
	}
	m := PitchMessageCommon{
		Contents: data,
		Length:   data[0],
		Type:     PitchMessageType(data[1]),
	}
	if m.Type != PitchMessageTypeTime && m.Type != PitchMessageTypeSymbolMapping && len(data) >= 6 {
		m.TimeOffset = binary.LittleEndian.Uint32(data[2:6])
	}
	return m
}
func (m *PitchMessageCommon) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	errs.PassE(&err)
	buf, err := b.AppendBytes(2)
	errs.CheckE(err)
	buf[0] = m.Length
	buf[1] = byte(m.Type)

	if m.Type != PitchMessageTypeTime {
		buf, err := b.AppendBytes(4)
		errs.CheckE(err)
		binary.LittleEndian.PutUint32(buf, m.TimeOffset)
	}
	return
}

/************************************************************************/
type PitchMessageUnknown struct {
	PitchMessageCommon
}

func (m *PitchMessageUnknown) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageUnknown{
		PitchMessageCommon: decodePitchMessage(data),
	}
	return nil
}

/************************************************************************/
type PitchMessageTime struct {
	PitchMessageCommon
	Time uint32
}

var _ packet.SecondsMessage = &PitchMessageTime{}

func (m *PitchMessageTime) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageTime{
		PitchMessageCommon: decodePitchMessage(data),
		Time:               binary.LittleEndian.Uint32(data[1:5]),
	}
	return nil
}
func (m *PitchMessageTime) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	errs.PassE(&err)
	errs.CheckE(m.PitchMessageCommon.SerializeTo(b, opts))
	buf, err := b.AppendBytes(4)
	errs.CheckE(err)
	binary.LittleEndian.PutUint32(buf, m.Time)
	return
}
func (m *PitchMessageTime) Seconds() int {
	return int(m.Time)
}

/************************************************************************/
type PitchMessageUnitClear struct {
	PitchMessageCommon
}

func (m *PitchMessageUnitClear) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageUnitClear{
		PitchMessageCommon: decodePitchMessage(data),
	}
	return nil
}
func (m *PitchMessageUnitClear) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	errs.PassE(&err)
	errs.CheckE(m.PitchMessageCommon.SerializeTo(b, opts))
	return
}

/************************************************************************/
type PitchMessageAddOrder struct {
	PitchMessageCommon
	OrderId       packet.OrderId
	Side          packet.MarketSide
	Size          uint32
	Symbol        packet.OptionId
	Price         packet.Price
	Flags         byte
	ParticipantId [4]byte
}

func (m *PitchMessageAddOrder) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageAddOrder{
		PitchMessageCommon: decodePitchMessage(data),
		OrderId:            packet.OrderIdFromUint64(binary.LittleEndian.Uint64(data[6:14])),
		Side:               packet.MarketSideFromByte(data[14]),
	}
	switch m.Type {
	case PitchMessageTypeAddOrderShort:
		m.Size = uint32(binary.LittleEndian.Uint16(data[15:17]))
		m.Symbol = parseSymbol(data[17:23])
		m.Price = packet.PriceFrom2Dec(int(binary.LittleEndian.Uint16(data[23:25])))
		m.Flags = data[25]
	case PitchMessageTypeAddOrderLong:
		m.Size = binary.LittleEndian.Uint32(data[15:19])
		m.Symbol = parseSymbol(data[19:25])
		m.Price = packet.PriceFrom4Dec(int(binary.LittleEndian.Uint64(data[25:33])))
		m.Flags = data[33]
	case PitchMessageTypeAddOrderExpanded:
		m.Size = binary.LittleEndian.Uint32(data[15:19])
		m.Symbol = parseSymbol(data[19:27])
		m.Price = packet.PriceFrom4Dec(int(binary.LittleEndian.Uint64(data[27:35])))
		m.Flags = data[35]
		copy(m.ParticipantId[:], data[36:40])
	}
	return nil
}

/************************************************************************/
type PitchMessageOrderExecuted struct {
	PitchMessageCommon
	OrderId     packet.OrderId
	Size        uint32
	ExecutionId uint64
}

func (m *PitchMessageOrderExecuted) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageOrderExecuted{
		PitchMessageCommon: decodePitchMessage(data),
		OrderId:            packet.OrderIdFromUint64(binary.LittleEndian.Uint64(data[6:14])),
		Size:               binary.LittleEndian.Uint32(data[14:18]),
		ExecutionId:        binary.LittleEndian.Uint64(data[18:26]),
	}
	return nil
}

/************************************************************************/
type PitchMessageOrderExecutedAtPriceSize struct {
	PitchMessageOrderExecuted
	RemainingSize uint32
	Price         packet.Price
}

func (m *PitchMessageOrderExecutedAtPriceSize) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageOrderExecutedAtPriceSize{
		PitchMessageOrderExecuted: PitchMessageOrderExecuted{
			PitchMessageCommon: decodePitchMessage(data),
			OrderId:            packet.OrderIdFromUint64(binary.LittleEndian.Uint64(data[6:14])),
			Size:               binary.LittleEndian.Uint32(data[14:18]),
			ExecutionId:        binary.LittleEndian.Uint64(data[22:30]),
		},
		RemainingSize: binary.LittleEndian.Uint32(data[18:22]),
		Price:         packet.PriceFrom4Dec(int(binary.LittleEndian.Uint64(data[30:38]))),
	}
	return nil
}

/************************************************************************/
type PitchMessageReduceSize struct {
	PitchMessageCommon
	OrderId packet.OrderId
	Size    uint32
}

func (m *PitchMessageReduceSize) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageReduceSize{
		PitchMessageCommon: decodePitchMessage(data),
		OrderId:            packet.OrderIdFromUint64(binary.LittleEndian.Uint64(data[6:14])),
	}
	switch m.Type {
	case PitchMessageTypeReduceSizeLong:
		m.Size = binary.LittleEndian.Uint32(data[14:18])
	case PitchMessageTypeReduceSizeShort:
		m.Size = uint32(binary.LittleEndian.Uint16(data[14:16]))
	}
	return nil
}

/************************************************************************/
type PitchMessageModifyOrder struct {
	PitchMessageCommon
	OrderId packet.OrderId
	Size    uint32
	Price   packet.Price
	Flags   byte
}

func (m *PitchMessageModifyOrder) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageModifyOrder{
		PitchMessageCommon: decodePitchMessage(data),
		OrderId:            packet.OrderIdFromUint64(binary.LittleEndian.Uint64(data[6:14])),
	}
	switch m.Type {
	case PitchMessageTypeModifyOrderLong:
		m.Size = binary.LittleEndian.Uint32(data[14:18])
		m.Price = packet.PriceFrom4Dec(int(binary.LittleEndian.Uint64(data[18:26])))
		m.Flags = data[26]
	case PitchMessageTypeModifyOrderShort:
		m.Size = uint32(binary.LittleEndian.Uint16(data[14:16]))
		m.Price = packet.PriceFrom2Dec(int(binary.LittleEndian.Uint16(data[16:18])))
		m.Flags = data[18]
	}
	return nil
}

/************************************************************************/
type PitchMessageDeleteOrder struct {
	PitchMessageCommon
	OrderId packet.OrderId
}

func (m *PitchMessageDeleteOrder) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageDeleteOrder{
		PitchMessageCommon: decodePitchMessage(data),
		OrderId:            packet.OrderIdFromUint64(binary.LittleEndian.Uint64(data[6:14])),
	}
	return nil
}

/************************************************************************/
type PitchMessageTrade struct {
	PitchMessageCommon
	OrderId     packet.OrderId
	Side        packet.MarketSide
	Size        uint32
	Symbol      packet.OptionId
	Price       packet.Price
	ExecutionId uint64
}

func (m *PitchMessageTrade) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageTrade{
		PitchMessageCommon: decodePitchMessage(data),
		OrderId:            packet.OrderIdFromUint64(binary.LittleEndian.Uint64(data[6:14])),
		Side:               packet.MarketSideFromByte(data[14]),
	}
	switch m.Type {
	case PitchMessageTypeTradeShort:
		m.Size = uint32(binary.LittleEndian.Uint16(data[15:16]))
		m.Symbol = parseSymbol(data[17:23])
		m.Price = packet.PriceFrom2Dec(int(binary.LittleEndian.Uint16(data[23:25])))
		m.ExecutionId = binary.LittleEndian.Uint64(data[25:33])
	case PitchMessageTypeTradeLong:
		m.Size = binary.LittleEndian.Uint32(data[15:19])
		m.Symbol = parseSymbol(data[19:25])
		m.Price = packet.PriceFrom4Dec(int(binary.LittleEndian.Uint64(data[25:33])))
		m.ExecutionId = binary.LittleEndian.Uint64(data[33:41])
	case PitchMessageTypeTradeExpanded:
		m.Size = binary.LittleEndian.Uint32(data[15:19])
		m.Symbol = parseSymbol(data[19:27])
		m.Price = packet.PriceFrom4Dec(int(binary.LittleEndian.Uint64(data[27:35])))
		m.ExecutionId = binary.LittleEndian.Uint64(data[35:43])
	}
	return nil
}

var _ packet.TradeMessage = &PitchMessageTrade{}

func (m *PitchMessageTrade) TradeInfo() (packet.OptionId, packet.Price, int) {
	return m.Symbol, m.Price, int(m.Size)
}

/************************************************************************/
type PitchMessageTradeBreak struct {
	PitchMessageCommon
	ExecutionId uint64
}

func (m *PitchMessageTradeBreak) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageTradeBreak{
		PitchMessageCommon: decodePitchMessage(data),
		ExecutionId:        binary.LittleEndian.Uint64(data[6:14]),
	}
	return nil
}

/************************************************************************/
type PitchMessageEndOfSession struct {
	PitchMessageCommon
}

func (m *PitchMessageEndOfSession) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageEndOfSession{
		PitchMessageCommon: decodePitchMessage(data),
	}
	return nil
}

/************************************************************************/
type PitchMessageSymbolMapping struct {
	PitchMessageCommon
	Symbol          packet.OptionId
	OsiSymbol       string
	SymbolCondition byte
}

func (m *PitchMessageSymbolMapping) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageSymbolMapping{
		PitchMessageCommon: decodePitchMessage(data),
		Symbol:             parseSymbol(data[2:8]),
		OsiSymbol:          string(data[8:29]),
		SymbolCondition:    data[29],
	}
	return nil
}

/************************************************************************/
type PitchMessageTradingStatus struct {
	PitchMessageCommon
	Symbol        packet.OptionId
	TradingStatus byte
	RegShoAction  byte
	Reserved      [2]byte
}

func (m *PitchMessageTradingStatus) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageTradingStatus{
		PitchMessageCommon: decodePitchMessage(data),
		Symbol:             parseSymbol(data[6:14]),
		TradingStatus:      data[14],
		RegShoAction:       data[15],
		Reserved:           [2]byte{data[16], data[17]},
	}
	return nil
}

/************************************************************************/
type PitchMessageAuctionUpdate struct {
	PitchMessageCommon
	Symbol           packet.OptionId
	AuctionType      byte
	ReferencePrice   packet.Price
	BuySize          uint32
	SellSize         uint32
	IndicativePrice  packet.Price
	AuctionOnlyPrice packet.Price
}

func (m *PitchMessageAuctionUpdate) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageAuctionUpdate{
		PitchMessageCommon: decodePitchMessage(data),
		Symbol:             parseSymbol(data[6:14]),
		AuctionType:        data[14],
		ReferencePrice:     packet.PriceFrom4Dec(int(binary.LittleEndian.Uint64(data[15:23]))),
		BuySize:            binary.LittleEndian.Uint32(data[23:27]),
		SellSize:           binary.LittleEndian.Uint32(data[27:31]),
		IndicativePrice:    packet.PriceFrom4Dec(int(binary.LittleEndian.Uint64(data[31:39]))),
		AuctionOnlyPrice:   packet.PriceFrom4Dec(int(binary.LittleEndian.Uint64(data[39:47]))),
	}
	return nil
}

/************************************************************************/
type PitchMessageAuctionSummary struct {
	PitchMessageCommon
	Symbol      packet.OptionId
	AuctionType byte
	Price       packet.Price
	Size        uint32
}

func (m *PitchMessageAuctionSummary) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageAuctionSummary{
		PitchMessageCommon: decodePitchMessage(data),
		Symbol:             parseSymbol(data[6:14]),
		AuctionType:        data[14],
		Price:              packet.PriceFrom4Dec(int(binary.LittleEndian.Uint64(data[15:23]))),
		Size:               binary.LittleEndian.Uint32(data[23:27]),
	}
	return nil
}

/************************************************************************/
type PitchMessageRetailPriceImprovement struct {
	PitchMessageCommon
	Symbol                 packet.OptionId
	RetailPriceImprovement byte
}

func (m *PitchMessageRetailPriceImprovement) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = PitchMessageRetailPriceImprovement{
		PitchMessageCommon:     decodePitchMessage(data),
		Symbol:                 parseSymbol(data[6:14]),
		RetailPriceImprovement: data[14],
	}
	return nil
}

/************************************************************************/
func parseSymbol(data []byte) packet.OptionId {
	errs.Check(len(data) >= 6)
	var b [8]byte
	copy(b[:], data)
	oid := packet.OptionIdFromUint64(binary.LittleEndian.Uint64(b[:]))
	return oid
}

/************************************************************************/

var PitchLayerFactory = &pitchLayerFactory{}

type pitchLayerFactory struct{}

var _ packet.DecodingLayerFactory = &pitchLayerFactory{}

func (f *pitchLayerFactory) Create(layerType gopacket.LayerType) gopacket.DecodingLayer {
	d := int(layerType - gopacket.LayerType(PITCH_LAYERS_BASE_NUM))
	if d < 0 || d > 255 {
		panic("FIXME")
		//return gopacket.LayerTypeZero // FIXME
	}
	m := PitchMessageTypeMetadata[d]
	errs.Check(m.LayerType == layerType)
	return m.CreateLayer()
}
func (f *pitchLayerFactory) SupportedLayers() gopacket.LayerClass {
	return LayerClassPitch
}
