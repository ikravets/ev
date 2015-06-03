// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package miax

import (
	"encoding/binary"
	"time"

	"code.google.com/p/gopacket"

	"my/errs"

	"my/itto/verify/packet"
)

var LayerTypeTom = gopacket.RegisterLayerType(11002, gopacket.LayerTypeMetadata{"Tom", gopacket.DecodeFunc(decodeTom)})

func decodeTom(data []byte, p gopacket.PacketBuilder) error {
	tomMessageType := TomMessageType(data[0])
	return tomMessageType.Decode(data, p)
}

/************************************************************************/

type TomMessageType uint8

func (a TomMessageType) Decode(data []byte, p gopacket.PacketBuilder) error {
	layer := TomMessageTypeMetadata[a].CreateLayer()
	if err := layer.DecodeFromBytes(data, p); err != nil {
		return err
	}
	p.AddLayer(layer)
	return p.NextDecoder(layer.NextLayerType())
}
func (a TomMessageType) String() string {
	return TomMessageTypeMetadata[a].Name
}
func (a TomMessageType) LayerType() gopacket.LayerType {
	return TomMessageTypeMetadata[a].LayerType
}

/************************************************************************/
const (
	TomMessageTypeUnknown               TomMessageType = 0 // not in spec, catch-all
	TomMessageTypeSystemTime            TomMessageType = '1'
	TomMessageTypeSeriesUpdate          TomMessageType = 'P'
	TomMessageTypeSystemState           TomMessageType = 'S'
	TomMessageTypeTomBidCompact         TomMessageType = 'B'
	TomMessageTypeTomOfferCompact       TomMessageType = 'O'
	TomMessageTypeTomBidWide            TomMessageType = 'W'
	TomMessageTypeTomOfferWide          TomMessageType = 'A'
	TomMessageTypeTrade                 TomMessageType = 'T'
	TomMessageTypeTradeCancel           TomMessageType = 'X'
	TomMessageTypeLiquiditySeeking      TomMessageType = 'L'
	TomMessageTypeUnderlyingTradeStatus TomMessageType = 'H'
)

var TomMessageTypeNames = [256]string{
	TomMessageTypeUnknown:               "TomUnknown",
	TomMessageTypeSystemTime:            "TomSystemTime",
	TomMessageTypeSeriesUpdate:          "TomSeriesUpdate",
	TomMessageTypeSystemState:           "TomSystemState",
	TomMessageTypeTomBidCompact:         "TomTomBidCompact",
	TomMessageTypeTomOfferCompact:       "TomTomOfferCompact",
	TomMessageTypeTomBidWide:            "TomTomBidWide",
	TomMessageTypeTomOfferWide:          "TomTomOfferWide",
	TomMessageTypeTrade:                 "TomTrade",
	TomMessageTypeTradeCancel:           "TomTradeCancel",
	TomMessageTypeLiquiditySeeking:      "TomLiquiditySeeking",
	TomMessageTypeUnderlyingTradeStatus: "TomUnderlyingTradeStatus",
}

var TomMessageCreators = [256]func() TomMessage{
	TomMessageTypeUnknown:               func() TomMessage { return &TomMessageUnknown{} },
	TomMessageTypeSystemTime:            func() TomMessage { return &TomMessageSystemTime{} },
	TomMessageTypeSeriesUpdate:          func() TomMessage { return &TomMessageSeriesUpdate{} },
	TomMessageTypeSystemState:           func() TomMessage { return &TomMessageSystemState{} },
	TomMessageTypeTomBidCompact:         func() TomMessage { return &TomMessageTom{} },
	TomMessageTypeTomOfferCompact:       func() TomMessage { return &TomMessageTom{} },
	TomMessageTypeTomBidWide:            func() TomMessage { return &TomMessageTom{} },
	TomMessageTypeTomOfferWide:          func() TomMessage { return &TomMessageTom{} },
	TomMessageTypeTrade:                 func() TomMessage { return &TomMessageTrade{} },
	TomMessageTypeTradeCancel:           func() TomMessage { return &TomMessageTradeCancel{} },
	TomMessageTypeLiquiditySeeking:      func() TomMessage { return &TomMessageLiquiditySeeking{} },
	TomMessageTypeUnderlyingTradeStatus: func() TomMessage { return &TomMessageUnderlyingTradeStatus{} },
}

type EnumMessageTypeMetadata struct {
	Name        string
	LayerType   gopacket.LayerType
	CreateLayer func() TomMessage
}

var TomMessageTypeMetadata [256]EnumMessageTypeMetadata
var LayerClassTom gopacket.LayerClass

const TOM_LAYERS_BASE_NUM = 11100

func init() {
	layerTypes := make([]gopacket.LayerType, 0, 256)
	for i := 0; i < 256; i++ {
		if TomMessageTypeNames[i] == "" {
			continue
		}
		tomMessageType := TomMessageType(i)
		layerTypeMetadata := gopacket.LayerTypeMetadata{
			Name:    TomMessageTypeNames[i],
			Decoder: tomMessageType,
		}
		layerType := gopacket.RegisterLayerType(TOM_LAYERS_BASE_NUM+i, layerTypeMetadata)
		layerTypes = append(layerTypes, layerType)
		creator := TomMessageCreators[i]
		createLayer := func() TomMessage {
			m := creator()
			m.Base().Type = tomMessageType
			return m
		}
		TomMessageTypeMetadata[i] = EnumMessageTypeMetadata{
			Name:        TomMessageTypeNames[i],
			LayerType:   layerType,
			CreateLayer: createLayer,
		}
	}
	for i := 0; i < 256; i++ {
		if TomMessageTypeMetadata[i].Name == "" {
			// unknown message type
			TomMessageTypeMetadata[i] = TomMessageTypeMetadata[TomMessageTypeUnknown]
		}
	}
	LayerClassTom = gopacket.NewLayerClass(layerTypes)
}

/************************************************************************/
type TomMessage interface {
	gopacket.DecodingLayer
	//embed gopacket.Layer by "inlining"
	//workaround for https://github.com/golang/go/issues/6977
	LayerType() gopacket.LayerType
	LayerContents() []byte

	Base() *TomMessageCommon
}

type TomMessageCommon struct {
	Type      TomMessageType
	Timestamp uint32
}

func (m *TomMessageCommon) CanDecode() gopacket.LayerClass {
	return m.LayerType()
}
func (m *TomMessageCommon) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}
func (m *TomMessageCommon) LayerContents() []byte {
	return nil
}
func (m *TomMessageCommon) LayerPayload() []byte {
	return nil
}
func (m *TomMessageCommon) LayerType() gopacket.LayerType {
	return m.Type.LayerType()
}

func (m *TomMessageCommon) Base() *TomMessageCommon {
	return m
}

func decodeTomMessage(data []byte) TomMessageCommon {
	m := TomMessageCommon{
		Type: TomMessageType(data[0]),
	}
	if m.Type != TomMessageTypeSystemTime && len(data) >= 5 {
		m.Timestamp = binary.LittleEndian.Uint32(data[1:5])
	}
	return m
}
func (m *TomMessageCommon) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	errs.PassE(&err)
	buf, err := b.AppendBytes(1)
	errs.CheckE(err)
	buf[0] = byte(m.Type)

	if m.Type != TomMessageTypeSystemTime {
		buf, err := b.AppendBytes(4)
		errs.CheckE(err)
		binary.LittleEndian.PutUint32(buf, m.Timestamp)
	}
	return
}

/************************************************************************/
type TomMessageUnknown struct {
	TomMessageCommon
	TypeChar string
}

func (m *TomMessageUnknown) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = TomMessageUnknown{
		TomMessageCommon: decodeTomMessage(data),
		TypeChar:         string(data[0]),
	}
	return nil
}

/************************************************************************/
type TomMessageSystemTime struct {
	TomMessageCommon
	Second uint32
}

func (m *TomMessageSystemTime) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = TomMessageSystemTime{
		TomMessageCommon: decodeTomMessage(data),
		Second:           binary.LittleEndian.Uint32(data[1:5]),
	}
	return nil
}
func (m *TomMessageSystemTime) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	errs.PassE(&err)
	errs.CheckE(m.TomMessageCommon.SerializeTo(b, opts))
	buf, err := b.AppendBytes(4)
	errs.CheckE(err)
	binary.LittleEndian.PutUint32(buf, m.Second)
	return
}

/************************************************************************/
type TomMessageSeriesUpdate struct {
	TomMessageCommon
	ProductId          packet.OptionId
	UnderlyingSymbol   string
	SecuritySymbol     string
	Expiration         string
	StrikePrice        packet.Price
	CallOrPut          byte // 'C' = call, 'P' = put
	OpeningTime        string
	ClosingTime        string
	Restricted         byte
	LongTerm           byte
	Active             byte
	MbboIncrement      byte
	LiquidityIncrement byte
	UnderlyingMarket   byte
	PriorityQuoteWidth int
	Reserved           [8]byte
}

func (m *TomMessageSeriesUpdate) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = TomMessageSeriesUpdate{
		TomMessageCommon:   decodeTomMessage(data),
		ProductId:          parseProductId(data[5:9]),
		UnderlyingSymbol:   string(data[9:20]),
		SecuritySymbol:     string(data[20:26]),
		Expiration:         string(data[26:34]),
		StrikePrice:        packet.PriceFrom4Dec(int(binary.LittleEndian.Uint32(data[34:38]))),
		CallOrPut:          data[38],
		OpeningTime:        string(data[39:47]),
		ClosingTime:        string(data[47:55]),
		Restricted:         data[56],
		LongTerm:           data[57],
		Active:             data[58],
		MbboIncrement:      data[59],
		LiquidityIncrement: data[60],
		UnderlyingMarket:   data[61],
		PriorityQuoteWidth: int(binary.LittleEndian.Uint32(data[62:66])),
		//Reserved:           data[66:74],
	}
	return nil
}

/************************************************************************/
type TomMessageSystemState struct {
	TomMessageCommon
	Version string
	Session uint32
	Status  byte
}

func (m *TomMessageSystemState) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = TomMessageSystemState{
		TomMessageCommon: decodeTomMessage(data),
		Version:          string(data[5:13]),
		Session:          binary.LittleEndian.Uint32(data[13:17]),
		Status:           data[18],
	}
	return nil
}

/************************************************************************/
type TomMessageTom struct {
	TomMessageCommon
	ProductId            packet.OptionId
	Price                packet.Price
	Size                 int
	PriorityCustomerSize int
	Condition            byte
	Side                 packet.MarketSide // inferred from message type
}

func (m *TomMessageTom) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = TomMessageTom{
		TomMessageCommon: decodeTomMessage(data),
		ProductId:        parseProductId(data[5:9]),
	}
	if m.Type == TomMessageTypeTomBidCompact || m.Type == TomMessageTypeTomOfferCompact {
		m.Price = packet.PriceFrom2Dec(int(binary.LittleEndian.Uint16(data[9:11])))
		m.Size = int(binary.LittleEndian.Uint16(data[11:13]))
		m.PriorityCustomerSize = int(binary.LittleEndian.Uint16(data[13:15]))
		m.Condition = data[15]
	} else if m.Type == TomMessageTypeTomBidWide || m.Type == TomMessageTypeTomOfferWide {
		m.Price = packet.PriceFrom4Dec(int(binary.LittleEndian.Uint32(data[9:13])))
		m.Size = int(binary.LittleEndian.Uint32(data[13:17]))
		m.PriorityCustomerSize = int(binary.LittleEndian.Uint32(data[17:21]))
		m.Condition = data[22]
	} else {
		panic("wrong message type")
	}
	if m.Type == TomMessageTypeTomBidCompact || m.Type == TomMessageTypeTomBidWide {
		m.Side = packet.MarketSideBid
	} else {
		m.Side = packet.MarketSideAsk
	}
	return nil
}

/************************************************************************/
type TomMessageTrade struct {
	TomMessageCommon
	ProductId           packet.OptionId
	TradeId             uint32
	Correction          uint8
	ReferenceTradeId    uint32
	ReferenceCorrection uint8
	Price               packet.Price
	Size                int
	TradeCondition      byte
}

func (m *TomMessageTrade) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = TomMessageTrade{
		TomMessageCommon:    decodeTomMessage(data),
		ProductId:           parseProductId(data[5:9]),
		TradeId:             binary.LittleEndian.Uint32(data[9:13]),
		Correction:          data[13],
		ReferenceTradeId:    binary.LittleEndian.Uint32(data[14:18]),
		ReferenceCorrection: data[18],
		Price:               packet.PriceFrom4Dec(int(binary.LittleEndian.Uint32(data[19:23]))),
		Size:                int(binary.LittleEndian.Uint32(data[23:27])),
		TradeCondition:      data[27],
	}
	return nil
}

/************************************************************************/
type TomMessageTradeCancel struct {
	TomMessageTrade
}

func (m *TomMessageTradeCancel) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = TomMessageTradeCancel{TomMessageTrade: TomMessageTrade{
		TomMessageCommon: decodeTomMessage(data),
		ProductId:        parseProductId(data[5:9]),
		TradeId:          binary.LittleEndian.Uint32(data[9:13]),
		Correction:       data[13],
		Price:            packet.PriceFrom4Dec(int(binary.LittleEndian.Uint32(data[14:18]))),
		Size:             int(binary.LittleEndian.Uint32(data[18:22])),
		TradeCondition:   data[22],
	}}
	return nil
}

/************************************************************************/
type TomMessageLiquiditySeeking struct {
	TomMessageCommon
	ProductId      packet.OptionId
	EventType      byte
	EventId        uint32
	Price          packet.Price
	Side           packet.MarketSide
	Quantity       [4]uint32
	AttributableId [4]byte
	Reserved       [8]byte
}

func (m *TomMessageLiquiditySeeking) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = TomMessageLiquiditySeeking{
		TomMessageCommon: decodeTomMessage(data),
		ProductId:        parseProductId(data[5:9]),
		EventType:        data[9],
		EventId:          binary.LittleEndian.Uint32(data[10:14]),
		Price:            packet.PriceFrom4Dec(int(binary.LittleEndian.Uint32(data[14:18]))),
		Side:             packet.MarketSideFromByte(data[18]),
		Quantity: [4]uint32{
			binary.LittleEndian.Uint32(data[19:23]),
			binary.LittleEndian.Uint32(data[23:27]),
			binary.LittleEndian.Uint32(data[27:31]),
			binary.LittleEndian.Uint32(data[31:35]),
		},
		AttributableId: [4]byte{data[35], data[36], data[37], data[38]},
		//Reserved:       data[39:43],
	}
	return nil
}

/************************************************************************/
type TomMessageUnderlyingTradeStatus struct {
	TomMessageCommon
	UnderlyingSymbol string
	Status           byte
	Reason           byte
	ExpectedTime     time.Time
}

func (m *TomMessageUnderlyingTradeStatus) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = TomMessageUnderlyingTradeStatus{
		TomMessageCommon: decodeTomMessage(data),
		UnderlyingSymbol: string(data[5:16]),
		Status:           data[16],
		Reason:           data[17],
		ExpectedTime:     time.Unix(int64(binary.LittleEndian.Uint32(data[18:22])), int64(binary.LittleEndian.Uint32(data[22:26]))),
	}
	return nil
}

/************************************************************************/
func parseProductId(data []byte) packet.OptionId {
	errs.Check(len(data) >= 4)
	return packet.OptionIdFromUint32(binary.LittleEndian.Uint32(data))
}

/************************************************************************/

var TomLayerFactory = &tomLayerFactory{}

type tomLayerFactory struct{}

var _ packet.DecodingLayerFactory = &tomLayerFactory{}

func (f *tomLayerFactory) Create(layerType gopacket.LayerType) gopacket.DecodingLayer {
	d := int(layerType - gopacket.LayerType(TOM_LAYERS_BASE_NUM))
	if d < 0 || d > 255 {
		panic("FIXME")
		//return gopacket.LayerTypeZero // FIXME
	}
	m := TomMessageTypeMetadata[d]
	errs.Check(m.LayerType == layerType)
	return m.CreateLayer()
}
func (f *tomLayerFactory) SupportedLayers() gopacket.LayerClass {
	return LayerClassTom
}
