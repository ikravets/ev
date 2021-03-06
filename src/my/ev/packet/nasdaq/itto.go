// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package nasdaq

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/ikravets/errs"

	"my/ev/packet"
)

var LayerTypeItto = gopacket.RegisterLayerType(10002, gopacket.LayerTypeMetadata{"Itto", gopacket.DecodeFunc(decodeItto)})

func decodeItto(data []byte, p gopacket.PacketBuilder) error {
	ittoMessageType := IttoMessageType(data[0])
	return ittoMessageType.Decode(data, p)
}

/************************************************************************/

type OrderSide struct {
	Side    packet.MarketSide
	RefNumD packet.OrderId
	Price   packet.Price
	Size    int
}
type ReplaceOrderSide struct {
	OrigRefNumD packet.OrderId
	OrderSide
}

/************************************************************************/
type IttoMessageType uint8

func (a IttoMessageType) Decode(data []byte, p gopacket.PacketBuilder) error {
	layer := IttoMessageTypeMetadata[a].CreateLayer()
	if err := layer.DecodeFromBytes(data, p); err != nil {
		return err
	}
	p.AddLayer(layer)
	return p.NextDecoder(layer.NextLayerType())
}
func (a IttoMessageType) String() string {
	return IttoMessageTypeMetadata[a].Name
}
func (a IttoMessageType) IsShort() bool {
	return IttoMessageTypeMetadata[a].IsShort
}
func (a IttoMessageType) LayerType() gopacket.LayerType {
	return IttoMessageTypeMetadata[a].LayerType
}

/************************************************************************/
const (
	IttoMessageTypeUnknown                     IttoMessageType = 0 // not in spec, catch-all
	IttoMessageTypeSeconds                     IttoMessageType = 'T'
	IttoMessageTypeSystemEvent                 IttoMessageType = 'S'
	IttoMessageTypeBaseReference               IttoMessageType = 'L'
	IttoMessageTypeOptionDirectory             IttoMessageType = 'R'
	IttoMessageTypeOptionTradingAction         IttoMessageType = 'H'
	IttoMessageTypeOptionOpen                  IttoMessageType = 'O'
	IttoMessageTypeAddOrderShort               IttoMessageType = 'a'
	IttoMessageTypeAddOrderLong                IttoMessageType = 'A'
	IttoMessageTypeAddQuoteShort               IttoMessageType = 'j'
	IttoMessageTypeAddQuoteLong                IttoMessageType = 'J'
	IttoMessageTypeSingleSideExecuted          IttoMessageType = 'E'
	IttoMessageTypeSingleSideExecutedWithPrice IttoMessageType = 'C'
	IttoMessageTypeOrderCancel                 IttoMessageType = 'X'
	IttoMessageTypeSingleSideReplaceShort      IttoMessageType = 'u'
	IttoMessageTypeSingleSideReplaceLong       IttoMessageType = 'U'
	IttoMessageTypeSingleSideDelete            IttoMessageType = 'D'
	IttoMessageTypeSingleSideUpdate            IttoMessageType = 'G'
	IttoMessageTypeQuoteReplaceShort           IttoMessageType = 'k'
	IttoMessageTypeQuoteReplaceLong            IttoMessageType = 'K'
	IttoMessageTypeQuoteDelete                 IttoMessageType = 'Y'
	IttoMessageTypeBlockSingleSideDelete       IttoMessageType = 'Z'
	IttoMessageTypeOptionsTrade                IttoMessageType = 'P'
	IttoMessageTypeOptionsCrossTrade           IttoMessageType = 'Q'
	IttoMessageTypeBrokenTrade                 IttoMessageType = 'B'
	IttoMessageTypeNoii                        IttoMessageType = 'I'
)

var IttoMessageTypeNames = [256]string{
	IttoMessageTypeUnknown:                     "IttoUnknown",
	IttoMessageTypeSeconds:                     "IttoSeconds",
	IttoMessageTypeSystemEvent:                 "IttoSystemEvent",
	IttoMessageTypeBaseReference:               "IttoBaseReference",
	IttoMessageTypeOptionDirectory:             "IttoOptionDirectory",
	IttoMessageTypeOptionTradingAction:         "IttoOptionTradingAction",
	IttoMessageTypeOptionOpen:                  "IttoOptionOpen",
	IttoMessageTypeAddOrderShort:               "IttoAddOrderShort",
	IttoMessageTypeAddOrderLong:                "IttoAddOrderLong",
	IttoMessageTypeAddQuoteShort:               "IttoAddQuoteShort",
	IttoMessageTypeAddQuoteLong:                "IttoAddQuoteLong",
	IttoMessageTypeSingleSideExecuted:          "IttoSingleSideExecuted",
	IttoMessageTypeSingleSideExecutedWithPrice: "IttoSingleSideExecutedWithPrice",
	IttoMessageTypeOrderCancel:                 "IttoOrderCancel",
	IttoMessageTypeSingleSideReplaceShort:      "IttoSingleSideReplaceShort",
	IttoMessageTypeSingleSideReplaceLong:       "IttoSingleSideReplaceLong",
	IttoMessageTypeSingleSideDelete:            "IttoSingleSideDelete",
	IttoMessageTypeSingleSideUpdate:            "IttoSingleSideUpdate",
	IttoMessageTypeQuoteReplaceShort:           "IttoQuoteReplaceShort",
	IttoMessageTypeQuoteReplaceLong:            "IttoQuoteReplaceLong",
	IttoMessageTypeQuoteDelete:                 "IttoQuoteDelete",
	IttoMessageTypeBlockSingleSideDelete:       "IttoBlockSingleSideDelete",
	IttoMessageTypeOptionsTrade:                "IttoOptionsTrade",
	IttoMessageTypeOptionsCrossTrade:           "IttoOptionsCrossTrade",
	IttoMessageTypeBrokenTrade:                 "IttoBrokenTrade",
	IttoMessageTypeNoii:                        "IttoNoii",
}

var IttoMessageCreators = [256]func() IttoMessage{
	IttoMessageTypeUnknown:                     func() IttoMessage { return &IttoMessageUnknown{} },
	IttoMessageTypeSeconds:                     func() IttoMessage { return &IttoMessageSeconds{} },
	IttoMessageTypeSystemEvent:                 func() IttoMessage { return &IttoMessageSystemEvent{} },
	IttoMessageTypeBaseReference:               func() IttoMessage { return &IttoMessageBaseReference{} },
	IttoMessageTypeOptionDirectory:             func() IttoMessage { return &IttoMessageOptionDirectory{} },
	IttoMessageTypeOptionTradingAction:         func() IttoMessage { return &IttoMessageOptionTradingAction{} },
	IttoMessageTypeOptionOpen:                  func() IttoMessage { return &IttoMessageOptionOpen{} },
	IttoMessageTypeAddOrderShort:               func() IttoMessage { return &IttoMessageAddOrder{} },
	IttoMessageTypeAddOrderLong:                func() IttoMessage { return &IttoMessageAddOrder{} },
	IttoMessageTypeAddQuoteShort:               func() IttoMessage { return &IttoMessageAddQuote{} },
	IttoMessageTypeAddQuoteLong:                func() IttoMessage { return &IttoMessageAddQuote{} },
	IttoMessageTypeSingleSideExecuted:          func() IttoMessage { return &IttoMessageSingleSideExecuted{} },
	IttoMessageTypeSingleSideExecutedWithPrice: func() IttoMessage { return &IttoMessageSingleSideExecutedWithPrice{} },
	IttoMessageTypeOrderCancel:                 func() IttoMessage { return &IttoMessageOrderCancel{} },
	IttoMessageTypeSingleSideReplaceShort:      func() IttoMessage { return &IttoMessageSingleSideReplace{} },
	IttoMessageTypeSingleSideReplaceLong:       func() IttoMessage { return &IttoMessageSingleSideReplace{} },
	IttoMessageTypeSingleSideDelete:            func() IttoMessage { return &IttoMessageSingleSideDelete{} },
	IttoMessageTypeSingleSideUpdate:            func() IttoMessage { return &IttoMessageSingleSideUpdate{} },
	IttoMessageTypeQuoteReplaceShort:           func() IttoMessage { return &IttoMessageQuoteReplace{} },
	IttoMessageTypeQuoteReplaceLong:            func() IttoMessage { return &IttoMessageQuoteReplace{} },
	IttoMessageTypeQuoteDelete:                 func() IttoMessage { return &IttoMessageQuoteDelete{} },
	IttoMessageTypeBlockSingleSideDelete:       func() IttoMessage { return &IttoMessageBlockSingleSideDelete{} },
	IttoMessageTypeOptionsTrade:                func() IttoMessage { return &IttoMessageOptionsTrade{} },
	IttoMessageTypeOptionsCrossTrade:           func() IttoMessage { return &IttoMessageOptionsCrossTrade{} },
	IttoMessageTypeBrokenTrade:                 func() IttoMessage { return &IttoMessageBrokenTrade{} },
	IttoMessageTypeNoii:                        func() IttoMessage { return &IttoMessageNoii{} },
}

var IttoMessageIsShort = [256]bool{
	IttoMessageTypeAddOrderShort:          true,
	IttoMessageTypeAddQuoteShort:          true,
	IttoMessageTypeSingleSideReplaceShort: true,
	IttoMessageTypeQuoteReplaceShort:      true,
}

type EnumMessageTypeMetadata struct {
	Name        string
	IsShort     bool
	LayerType   gopacket.LayerType
	CreateLayer func() IttoMessage
}

var IttoMessageTypeMetadata [256]EnumMessageTypeMetadata
var LayerClassItto gopacket.LayerClass

const ITTO_LAYERS_BASE_NUM = 10100

func init() {
	layerTypes := make([]gopacket.LayerType, 0, 256)
	for i := 0; i < 256; i++ {
		if IttoMessageTypeNames[i] == "" {
			continue
		}
		ittoMessageType := IttoMessageType(i)
		layerTypeMetadata := gopacket.LayerTypeMetadata{
			Name:    IttoMessageTypeNames[i],
			Decoder: ittoMessageType,
		}
		layerType := gopacket.RegisterLayerType(ITTO_LAYERS_BASE_NUM+i, layerTypeMetadata)
		layerTypes = append(layerTypes, layerType)
		creator := IttoMessageCreators[i]
		createLayer := func() IttoMessage {
			m := creator()
			m.Base().Type = ittoMessageType
			return m
		}
		IttoMessageTypeMetadata[i] = EnumMessageTypeMetadata{
			Name:        IttoMessageTypeNames[i],
			IsShort:     IttoMessageIsShort[i],
			LayerType:   layerType,
			CreateLayer: createLayer,
		}
	}
	for i := 0; i < 256; i++ {
		if IttoMessageTypeMetadata[i].Name == "" {
			// unknown message type
			IttoMessageTypeMetadata[i] = IttoMessageTypeMetadata[IttoMessageTypeUnknown]
		}
	}
	LayerClassItto = gopacket.NewLayerClass(layerTypes)
}

/************************************************************************/
type IttoMessage interface {
	packet.ExchangeMessage
	gopacket.DecodingLayer
	//embed gopacket.Layer by "inlining"
	//workaround for https://github.com/golang/go/issues/6977
	LayerType() gopacket.LayerType
	LayerContents() []byte

	Base() *IttoMessageCommon
}

type IttoMessageCommon struct {
	Type      IttoMessageType
	Timestamp uint32
}

func (m *IttoMessageCommon) CanDecode() gopacket.LayerClass {
	return m.LayerType()
}
func (m *IttoMessageCommon) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}
func (m *IttoMessageCommon) LayerContents() []byte {
	return nil
}
func (m *IttoMessageCommon) LayerPayload() []byte {
	return nil
}
func (m *IttoMessageCommon) LayerType() gopacket.LayerType {
	return m.Type.LayerType()
}

func (m *IttoMessageCommon) Base() *IttoMessageCommon {
	return m
}
func (m *IttoMessageCommon) Nanoseconds() int {
	return int(m.Timestamp)
}
func (m *IttoMessageCommon) OptionId() packet.OptionId {
	return packet.OptionIdUnknown
}

func decodeIttoMessage(data []byte) IttoMessageCommon {
	m := IttoMessageCommon{
		Type: IttoMessageType(data[0]),
	}
	if m.Type != IttoMessageTypeSeconds && len(data) >= 5 {
		m.Timestamp = binary.BigEndian.Uint32(data[1:5])
	}
	return m
}
func (m *IttoMessageCommon) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	defer errs.PassE(&err)
	buf, err := b.AppendBytes(1)
	errs.CheckE(err)
	buf[0] = byte(m.Type)

	if m.Type != IttoMessageTypeSeconds {
		buf, err := b.AppendBytes(4)
		errs.CheckE(err)
		binary.BigEndian.PutUint32(buf, m.Timestamp)
	}
	return
}

/************************************************************************/
type IttoMessageUnknown struct {
	IttoMessageCommon
	TypeChar string
}

func (m *IttoMessageUnknown) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageUnknown{
		IttoMessageCommon: decodeIttoMessage(data),
		TypeChar:          string(data[0]),
	}
	return nil
}

/************************************************************************/
type IttoMessageSeconds struct {
	IttoMessageCommon
	Second uint32
}

var _ packet.SecondsMessage = &IttoMessageSeconds{}

func (m *IttoMessageSeconds) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageSeconds{
		IttoMessageCommon: decodeIttoMessage(data),
		Second:            binary.BigEndian.Uint32(data[1:5]),
	}
	return nil
}
func (m *IttoMessageSeconds) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	defer errs.PassE(&err)
	errs.CheckE(m.IttoMessageCommon.SerializeTo(b, opts))
	buf, err := b.AppendBytes(4)
	errs.CheckE(err)
	binary.BigEndian.PutUint32(buf, m.Second)
	return
}
func (m *IttoMessageSeconds) Seconds() int {
	return int(m.Second)
}

/************************************************************************/
type IttoMessageSystemEvent struct {
	IttoMessageCommon
	EventCode byte
}

func (m *IttoMessageSystemEvent) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageSystemEvent{
		IttoMessageCommon: decodeIttoMessage(data),
		EventCode:         data[5],
	}
	return nil
}

/************************************************************************/
type IttoMessageBaseReference struct {
	IttoMessageCommon
	BaseRefNum uint64
}

func (m *IttoMessageBaseReference) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageBaseReference{
		IttoMessageCommon: decodeIttoMessage(data),
		BaseRefNum:        binary.BigEndian.Uint64(data[5:13]),
	}
	return nil
}
func (m *IttoMessageBaseReference) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	defer errs.PassE(&err)
	errs.CheckE(m.IttoMessageCommon.SerializeTo(b, opts))
	buf, err := b.AppendBytes(8)
	errs.CheckE(err)
	binary.BigEndian.PutUint64(buf, m.BaseRefNum)
	return
}

/************************************************************************/
type IttoMessageOptionDirectory struct {
	IttoMessageCommon
	OId              packet.OptionId
	Symbol           string
	Expiration       time.Time
	StrikePrice      int
	OType            byte
	Source           uint8
	UnderlyingSymbol string
	ClosingType      byte
	Tradable         byte
	MPV              byte
}

func (m *IttoMessageOptionDirectory) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageOptionDirectory{
		IttoMessageCommon: decodeIttoMessage(data),
		OId:               packet.OptionIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
		Symbol:            string(data[9:15]),
		Expiration:        time.Date(2000+int(data[15]), time.Month(data[16]), int(data[17]), 0, 0, 0, 0, time.Local),
		StrikePrice:       int(binary.BigEndian.Uint32(data[18:22])),
		OType:             data[22],
		Source:            data[23],
		UnderlyingSymbol:  string(data[24:37]),
		ClosingType:       data[37],
		Tradable:          data[38],
		MPV:               data[39],
	}
	return nil
}
func (m *IttoMessageOptionDirectory) OptionId() packet.OptionId {
	return m.OId
}

/************************************************************************/
type IttoMessageOptionTradingAction struct {
	IttoMessageCommon
	OId   packet.OptionId
	State byte
}

func (m *IttoMessageOptionTradingAction) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageOptionTradingAction{
		IttoMessageCommon: decodeIttoMessage(data),
		OId:               packet.OptionIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
		State:             data[9],
	}
	return nil
}
func (m *IttoMessageOptionTradingAction) OptionId() packet.OptionId {
	return m.OId
}

/************************************************************************/
type IttoMessageOptionOpen struct {
	IttoMessageCommon
	OId       packet.OptionId
	OpenState byte
}

func (m *IttoMessageOptionOpen) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageOptionOpen{
		IttoMessageCommon: decodeIttoMessage(data),
		OId:               packet.OptionIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
		OpenState:         data[9],
	}
	return nil
}
func (m *IttoMessageOptionOpen) OptionId() packet.OptionId {
	return m.OId
}

/************************************************************************/
type IttoMessageAddOrder struct {
	IttoMessageCommon
	OId packet.OptionId
	OrderSide
}

func (m *IttoMessageAddOrder) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageAddOrder{
		IttoMessageCommon: decodeIttoMessage(data),
		OrderSide: OrderSide{
			RefNumD: packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
			Side:    packet.MarketSideFromByte(data[9]),
		},
		OId: packet.OptionIdFromUint32(binary.BigEndian.Uint32(data[10:14])),
	}
	if m.Type.IsShort() {
		m.Price = packet.PriceFrom2Dec(int(binary.BigEndian.Uint16(data[14:16])))
		m.Size = int(binary.BigEndian.Uint16(data[16:18]))
	} else {
		m.Price = packet.PriceFrom4Dec(int(binary.BigEndian.Uint32(data[14:18])))
		m.Size = int(binary.BigEndian.Uint32(data[18:22]))
	}
	return nil
}
func (m *IttoMessageAddOrder) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	defer errs.PassE(&err)
	errs.CheckE(m.IttoMessageCommon.SerializeTo(b, opts))
	buf, err := b.AppendBytes(9)
	errs.CheckE(err)
	binary.BigEndian.PutUint32(buf, m.RefNumD.ToUint32())
	buf[4], err = m.Side.ToByte()
	errs.CheckE(err)
	binary.BigEndian.PutUint32(buf[5:9], m.OId.ToUint32())
	if m.Type.IsShort() {
		buf, err := b.AppendBytes(4)
		errs.CheckE(err)
		binary.BigEndian.PutUint16(buf, uint16(packet.PriceTo2Dec(m.Price)))
		binary.BigEndian.PutUint16(buf[2:], uint16(m.Size))
	} else {
		buf, err := b.AppendBytes(8)
		errs.CheckE(err)
		binary.BigEndian.PutUint32(buf, uint32(packet.PriceTo4Dec(m.Price)))
		binary.BigEndian.PutUint32(buf[4:], uint32(m.Size))
	}
	return
}
func (m *IttoMessageAddOrder) OptionId() packet.OptionId {
	return m.OId
}

/************************************************************************/
type IttoMessageAddQuote struct {
	IttoMessageCommon
	OId packet.OptionId
	Bid OrderSide
	Ask OrderSide
}

func (m *IttoMessageAddQuote) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageAddQuote{
		IttoMessageCommon: decodeIttoMessage(data),
		Bid:               OrderSide{Side: packet.MarketSideBid, RefNumD: packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[5:9]))},
		Ask:               OrderSide{Side: packet.MarketSideAsk, RefNumD: packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[9:13]))},
		OId:               packet.OptionIdFromUint32(binary.BigEndian.Uint32(data[13:17])),
	}
	if m.Type.IsShort() {
		m.Bid.Price = packet.PriceFrom2Dec(int(binary.BigEndian.Uint16(data[17:19])))
		m.Bid.Size = int(binary.BigEndian.Uint16(data[19:21]))
		m.Ask.Price = packet.PriceFrom2Dec(int(binary.BigEndian.Uint16(data[21:23])))
		m.Ask.Size = int(binary.BigEndian.Uint16(data[23:25]))
	} else {
		m.Bid.Price = packet.PriceFrom4Dec(int(binary.BigEndian.Uint32(data[17:21])))
		m.Bid.Size = int(binary.BigEndian.Uint32(data[21:25]))
		m.Ask.Price = packet.PriceFrom4Dec(int(binary.BigEndian.Uint32(data[25:29])))
		m.Ask.Size = int(binary.BigEndian.Uint32(data[29:33]))
	}
	return nil
}
func (m *IttoMessageAddQuote) OptionId() packet.OptionId {
	return m.OId
}

/************************************************************************/
type IttoMessageSingleSideExecuted struct {
	IttoMessageCommon
	OrigRefNumD packet.OrderId
	Size        int
	Cross       uint32
	Match       uint32
}

func (m *IttoMessageSingleSideExecuted) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageSingleSideExecuted{
		IttoMessageCommon: decodeIttoMessage(data),
		OrigRefNumD:       packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
		Size:              int(binary.BigEndian.Uint32(data[9:13])),
		Cross:             binary.BigEndian.Uint32(data[13:17]),
		Match:             binary.BigEndian.Uint32(data[17:21]),
	}
	return nil
}

/************************************************************************/
type IttoMessageSingleSideExecutedWithPrice struct {
	IttoMessageSingleSideExecuted
	Printable byte
	Price     packet.Price
}

func (m *IttoMessageSingleSideExecutedWithPrice) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageSingleSideExecutedWithPrice{
		IttoMessageSingleSideExecuted: IttoMessageSingleSideExecuted{
			IttoMessageCommon: decodeIttoMessage(data),
			OrigRefNumD:       packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
			Cross:             binary.BigEndian.Uint32(data[9:13]),
			Match:             binary.BigEndian.Uint32(data[13:17]),
			Size:              int(binary.BigEndian.Uint32(data[22:26])),
		},
		Printable: data[17],
		Price:     packet.PriceFrom4Dec(int(binary.BigEndian.Uint32(data[18:22]))),
	}
	return nil
}

/************************************************************************/
type IttoMessageOrderCancel struct {
	IttoMessageCommon
	OrigRefNumD packet.OrderId
	Size        int
}

func (m *IttoMessageOrderCancel) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageOrderCancel{
		IttoMessageCommon: decodeIttoMessage(data),
		OrigRefNumD:       packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
		Size:              int(binary.BigEndian.Uint32(data[9:13])),
	}
	return nil
}

/************************************************************************/
type IttoMessageSingleSideReplace struct {
	IttoMessageCommon
	ReplaceOrderSide
}

func (m *IttoMessageSingleSideReplace) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageSingleSideReplace{
		IttoMessageCommon: decodeIttoMessage(data),
		ReplaceOrderSide: ReplaceOrderSide{
			OrigRefNumD: packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
			OrderSide:   OrderSide{RefNumD: packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[9:13]))},
		},
	}
	if m.Type.IsShort() {
		m.Price = packet.PriceFrom2Dec(int(binary.BigEndian.Uint16(data[13:15])))
		m.Size = int(binary.BigEndian.Uint16(data[15:17]))
	} else {
		m.Price = packet.PriceFrom4Dec(int(binary.BigEndian.Uint32(data[13:17])))
		m.Size = int(binary.BigEndian.Uint32(data[17:21]))
	}
	return nil
}

/************************************************************************/
type IttoMessageSingleSideDelete struct {
	IttoMessageCommon
	OrigRefNumD packet.OrderId
}

func (m *IttoMessageSingleSideDelete) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageSingleSideDelete{
		IttoMessageCommon: decodeIttoMessage(data),
		OrigRefNumD:       packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
	}
	return nil
}

/************************************************************************/
type IttoMessageSingleSideUpdate struct {
	IttoMessageCommon
	OrderSide
	Reason byte
}

func (m *IttoMessageSingleSideUpdate) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageSingleSideUpdate{
		IttoMessageCommon: decodeIttoMessage(data),
		OrderSide: OrderSide{
			RefNumD: packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
			Price:   packet.PriceFrom4Dec(int(binary.BigEndian.Uint32(data[10:14]))),
			Size:    int(binary.BigEndian.Uint32(data[14:18])),
		},
		Reason: data[9],
	}
	return nil
}

/************************************************************************/
type IttoMessageQuoteReplace struct {
	IttoMessageCommon
	Bid ReplaceOrderSide
	Ask ReplaceOrderSide
}

func (m *IttoMessageQuoteReplace) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageQuoteReplace{
		IttoMessageCommon: decodeIttoMessage(data),
		Bid: ReplaceOrderSide{
			OrigRefNumD: packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
			OrderSide:   OrderSide{Side: packet.MarketSideBid, RefNumD: packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[9:13]))},
		},
		Ask: ReplaceOrderSide{
			OrigRefNumD: packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[13:17])),
			OrderSide:   OrderSide{Side: packet.MarketSideAsk, RefNumD: packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[17:21]))},
		},
	}
	if m.Type.IsShort() {
		m.Bid.Price = packet.PriceFrom2Dec(int(binary.BigEndian.Uint16(data[21:23])))
		m.Bid.Size = int(binary.BigEndian.Uint16(data[23:25]))
		m.Ask.Price = packet.PriceFrom2Dec(int(binary.BigEndian.Uint16(data[25:27])))
		m.Ask.Size = int(binary.BigEndian.Uint16(data[27:29]))
	} else {
		m.Bid.Price = packet.PriceFrom4Dec(int(binary.BigEndian.Uint32(data[21:25])))
		m.Bid.Size = int(binary.BigEndian.Uint32(data[25:29]))
		m.Ask.Price = packet.PriceFrom4Dec(int(binary.BigEndian.Uint32(data[29:33])))
		m.Ask.Size = int(binary.BigEndian.Uint32(data[33:37]))
	}
	return nil
}

/************************************************************************/
type IttoMessageQuoteDelete struct {
	IttoMessageCommon
	BidOrigRefNumD packet.OrderId
	AskOrigRefNumD packet.OrderId
}

func (m *IttoMessageQuoteDelete) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageQuoteDelete{
		IttoMessageCommon: decodeIttoMessage(data),
		BidOrigRefNumD:    packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
		AskOrigRefNumD:    packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[9:13])),
	}
	return nil
}

/************************************************************************/
type IttoMessageBlockSingleSideDelete struct {
	IttoMessageCommon
	Number   int
	RefNumDs []packet.OrderId
}

func (m *IttoMessageBlockSingleSideDelete) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageBlockSingleSideDelete{
		IttoMessageCommon: decodeIttoMessage(data),
		Number:            int(binary.BigEndian.Uint16(data[5:9])),
	}
	m.RefNumDs = make([]packet.OrderId, m.Number)
	for i := 0; i < m.Number; i++ {
		off := 7 + 4*i
		m.RefNumDs[i] = packet.OrderIdFromUint32(binary.BigEndian.Uint32(data[off : off+4]))
	}
	return nil
}
func (m *IttoMessageBlockSingleSideDelete) String() string {
	// similar to default gopacket.LayerString format
	// {Type=IttoBlockSingleSideDelete Timestamp=450423694 Number=286 RefNumDs=[..286..]}
	// but with expanded RefNumDs slice
	var bb bytes.Buffer
	fmt.Fprintf(&bb, "{Type=IttoBlockSingleSideDelete Timestamp=%d Number=%d RefNumDs=[", m.Timestamp, m.Number)
	for i, ref := range m.RefNumDs {
		if i > 0 {
			bb.WriteString(" ")
		}
		bb.WriteString(ref.String())
	}
	bb.WriteString("]}")
	return bb.String()
}

/************************************************************************/
type IttoMessageOptionsTrade struct {
	IttoMessageCommon
	Side  packet.MarketSide
	OId   packet.OptionId
	Cross uint32
	Match uint32
	Price packet.Price
	Size  int
}

func (m *IttoMessageOptionsTrade) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageOptionsTrade{
		IttoMessageCommon: decodeIttoMessage(data),
		Side:              packet.MarketSideFromByte(data[5]),
		OId:               packet.OptionIdFromUint32(binary.BigEndian.Uint32(data[6:10])),
		Cross:             binary.BigEndian.Uint32(data[10:14]),
		Match:             binary.BigEndian.Uint32(data[14:18]),
		Price:             packet.PriceFrom4Dec(int(binary.BigEndian.Uint32(data[18:22]))),
		Size:              int(binary.BigEndian.Uint32(data[22:26])),
	}
	return nil
}
func (m *IttoMessageOptionsTrade) OptionId() packet.OptionId {
	return m.OId
}

var _ packet.TradeMessage = &IttoMessageOptionsTrade{}

func (m *IttoMessageOptionsTrade) TradeInfo() (packet.OptionId, packet.Price, int) {
	return m.OId, m.Price, m.Size
}

/************************************************************************/
type IttoMessageOptionsCrossTrade struct {
	IttoMessageOptionsTrade
	CrossType byte
}

func (m *IttoMessageOptionsCrossTrade) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageOptionsCrossTrade{
		IttoMessageOptionsTrade: IttoMessageOptionsTrade{
			IttoMessageCommon: decodeIttoMessage(data),
			OId:               packet.OptionIdFromUint32(binary.BigEndian.Uint32(data[5:9])),
			Cross:             binary.BigEndian.Uint32(data[9:13]),
			Match:             binary.BigEndian.Uint32(data[13:17]),
			Price:             packet.PriceFrom4Dec(int(binary.BigEndian.Uint32(data[18:22]))),
			Size:              int(binary.BigEndian.Uint32(data[22:26])),
		},
		CrossType: data[17],
	}
	return nil
}

/************************************************************************/
type IttoMessageBrokenTrade struct {
	IttoMessageCommon
	Cross uint32
	Match uint32
}

func (m *IttoMessageBrokenTrade) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageBrokenTrade{
		IttoMessageCommon: decodeIttoMessage(data),
		Cross:             binary.BigEndian.Uint32(data[5:9]),
		Match:             binary.BigEndian.Uint32(data[9:13]),
	}
	return nil
}

/************************************************************************/
type IttoMessageNoii struct {
	IttoMessageCommon
	AuctionId   uint32
	AuctionType byte
	Size        uint32
	OId         packet.OptionId
	Imbalance   OrderSide
}

func (m *IttoMessageNoii) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	*m = IttoMessageNoii{
		IttoMessageCommon: decodeIttoMessage(data),
		AuctionId:         binary.BigEndian.Uint32(data[5:9]),
		AuctionType:       data[9],
		Size:              binary.BigEndian.Uint32(data[10:14]),
		Imbalance: OrderSide{
			Side:  packet.MarketSideFromByte(data[14]),
			Price: packet.PriceFrom4Dec(int(binary.BigEndian.Uint32(data[19:23]))),
			Size:  int(binary.BigEndian.Uint32(data[23:27])),
		},
		OId: packet.OptionIdFromUint32(binary.BigEndian.Uint32(data[15:19])),
	}
	return nil
}

/************************************************************************/
var IttoLayerFactory = &ittoLayerFactory{}

type ittoLayerFactory struct{}

var _ packet.DecodingLayerFactory = &ittoLayerFactory{}

func (f *ittoLayerFactory) Create(layerType gopacket.LayerType) gopacket.DecodingLayer {
	d := int(layerType - gopacket.LayerType(ITTO_LAYERS_BASE_NUM))
	if d < 0 || d > 255 {
		panic("FIXME")
		//return gopacket.LayerTypeZero // FIXME
	}
	m := IttoMessageTypeMetadata[d]
	errs.Check(m.LayerType == layerType)
	return m.CreateLayer()
}
func (f *ittoLayerFactory) SupportedLayers() gopacket.LayerClass {
	return LayerClassItto
}
