// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package itto

import (
	"encoding/binary"
	"strconv"

	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/layers"
)

type IttoDecoder struct {
}

var LayerTypeItto = gopacket.RegisterLayerType(10002, gopacket.LayerTypeMetadata{"Itto", IttoDecoder{}})

type Itto struct {
	Type byte
}

func (m *Itto) LayerType() gopacket.LayerType {
	return LayerTypeItto
}

func (m *Itto) LayerContents() []byte {
	return nil
}

func (m *Itto) LayerPayload() []byte {
	return nil
}

func (d IttoDecoder) Decode(data []byte, p gopacket.PacketBuilder) error {
	if false {
		itto := &Itto{
			Type: data[0],
		}
		p.AddLayer(itto)
		return nil
	} else {
		//return p.NextDecoder(ittoMessageTypePrefixDecoder)
		return decodeWithIttoMessageTypePrefix(data, p)
	}
}

var ittoMessageTypePrefixDecoder = gopacket.DecodeFunc(decodeWithIttoMessageTypePrefix)

func decodeWithIttoMessageTypePrefix(data []byte, p gopacket.PacketBuilder) error {
	chunkType := IttoMessageType(data[0])
	return chunkType.Decode(data, p)
}

/************************************************************************/
const (
	PriceScale      = 10000
	PriceScaleShort = 100
)

func PriceShortToLong(shortPrice int) int {
	return shortPrice * PriceScale / PriceScaleShort
}

type MarketSide byte

const (
	MarketSideUnknown MarketSide = 0
	MarketSideBid     MarketSide = 'B'
	MarketSideAsk     MarketSide = 'A'
)

func (ms MarketSide) String() string {
	switch ms {
	case MarketSideBid:
		return "B"
	case MarketSideAsk:
		return "A"
	default:
		return "?"
	}
}

func MarketSideParse(b byte) MarketSide {
	switch b {
	case 'B':
		return MarketSideBid
	case 'A', 'S':
		return MarketSideAsk
	default:
		return MarketSideUnknown
	}
}

type RefNumDelta struct {
	delta uint32
	isNew bool
}

func NewRefNumDelta(delta uint32) RefNumDelta {
	return RefNumDelta{delta: delta, isNew: true}
}
func OrigRefNumDelta(delta uint32) RefNumDelta {
	return RefNumDelta{delta: delta}
}
func (r RefNumDelta) String() string {
	return strconv.FormatUint(uint64(r.delta), 10)
}
func (r RefNumDelta) Delta() uint32 {
	return r.delta
}

type OptionId uint32

func (oid OptionId) Valid() bool {
	return oid != 0
}
func (oid OptionId) Invalid() bool {
	return oid == 0
}

type OrderSide struct {
	Side    MarketSide
	RefNumD RefNumDelta
	Price   int
	Size    int
}
type ReplaceOrderSide struct {
	OrigRefNumD RefNumDelta
	OrderSide
}

/************************************************************************/
type IttoMessageType uint8

const (
	IttoMessageTypeSeconds                     IttoMessageType = 'T'
	IttoMessageTypeSystemEvent                 IttoMessageType = 'S' // TODO
	IttoMessageTypeBaseReference               IttoMessageType = 'L' // TODO
	IttoMessageTypeOptionDirectory             IttoMessageType = 'R' // TODO
	IttoMessageTypeOptionsTradingAction        IttoMessageType = 'H' // TODO
	IttoMessageTypeOptionsOpen                 IttoMessageType = 'O' // TODO
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
	IttoMessageTypeOptionsTrade                IttoMessageType = 'P' // TODO
	IttoMessageTypeOptionsCrossTrade           IttoMessageType = 'Q' // TODO
	IttoMessageTypeBrokenTrade                 IttoMessageType = 'B' // TODO
	IttoMessageTypeNoii                        IttoMessageType = 'I'
)

func (a IttoMessageType) Decode(data []byte, p gopacket.PacketBuilder) error {
	return IttoMessageTypeMetadata[a].DecodeWith.Decode(data, p)
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
type EnumMessageTypeMetadata struct {
	layers.EnumMetadata
	IsShort bool
}

var IttoMessageTypeMetadata [265]EnumMessageTypeMetadata
var LayerClassItto gopacket.LayerClass

func init() {
	for i := 0; i < 256; i++ {
		/*
			IttoMessageTypeMetadata[i] = layers.EnumMetadata{
				DecodeWith: errorFunc(fmt.Sprintf("Unable to decode ITTO message type %d (%c)", i, i)),
				Name:       fmt.Sprintf("UnknownIttoMessage(%d)", i),
			}
		*/
		IttoMessageTypeMetadata[i] = EnumMessageTypeMetadata{
			EnumMetadata: layers.EnumMetadata{
				DecodeWith: gopacket.DecodeFunc(decodeIttoUnknown),
				Name:       "Unknown",
				LayerType:  LayerTypeIttoUnknown,
			},
		}
	}
	IttoMessageTypeMetadata[IttoMessageTypeSeconds] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoSeconds),
			Name:       "Seconds",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeSeconds), gopacket.LayerTypeMetadata{"IttoSeconds", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeBaseReference] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoBaseReference),
			Name:       "BaseReference",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeBaseReference), gopacket.LayerTypeMetadata{"IttoBaseReference", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeAddOrderShort] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoAddOrder),
			Name:       "AddOrderShort",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeAddOrderShort), gopacket.LayerTypeMetadata{"IttoAddOrderShort", nil}),
		},
		IsShort: true,
	}
	IttoMessageTypeMetadata[IttoMessageTypeAddOrderLong] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoAddOrder),
			Name:       "AddOrderLong",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeAddOrderLong), gopacket.LayerTypeMetadata{"IttoAddOrderLong", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeAddQuoteShort] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoAddQuote),
			Name:       "AddQuoteShort",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeAddQuoteShort), gopacket.LayerTypeMetadata{"IttoAddQuoteShort", nil}),
		},
		IsShort: true,
	}
	IttoMessageTypeMetadata[IttoMessageTypeAddQuoteLong] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoAddQuote),
			Name:       "AddQuoteLong",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeAddQuoteLong), gopacket.LayerTypeMetadata{"IttoAddQuoteLong", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeSingleSideExecuted] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoSingleSideExecuted),
			Name:       "SingleSideExecuted",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeSingleSideExecuted), gopacket.LayerTypeMetadata{"IttoSingleSideExecuted", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeSingleSideExecutedWithPrice] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoSingleSideExecutedWithPrice),
			Name:       "SingleSideExecutedWithPrice",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeSingleSideExecutedWithPrice), gopacket.LayerTypeMetadata{"IttoSingleSideExecutedWithPrice", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeOrderCancel] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoOrderCancel),
			Name:       "OrderCancel",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeOrderCancel), gopacket.LayerTypeMetadata{"IttoOrderCancel", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeSingleSideReplaceShort] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoSingleSideReplace),
			Name:       "SingleSideReplaceShort",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeSingleSideReplaceShort), gopacket.LayerTypeMetadata{"IttoSingleSideReplaceShort", nil}),
		},
		IsShort: true,
	}
	IttoMessageTypeMetadata[IttoMessageTypeSingleSideReplaceLong] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoSingleSideReplace),
			Name:       "SingleSideReplaceLong",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeSingleSideReplaceLong), gopacket.LayerTypeMetadata{"IttoSingleSideReplaceLong", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeSingleSideDelete] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoSingleSideDelete),
			Name:       "SingleSideDelete",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeSingleSideDelete), gopacket.LayerTypeMetadata{"IttoSingleSideDelete", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeSingleSideUpdate] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoSingleSideUpdate),
			Name:       "SingleSideUpdate",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeSingleSideUpdate), gopacket.LayerTypeMetadata{"IttoSingleSideUpdate", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeQuoteReplaceShort] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoQuoteReplace),
			Name:       "QuoteReplaceShort",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeQuoteReplaceShort), gopacket.LayerTypeMetadata{"IttoQuoteReplaceShort", nil}),
		},
		IsShort: true,
	}
	IttoMessageTypeMetadata[IttoMessageTypeQuoteReplaceLong] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoQuoteReplace),
			Name:       "QuoteReplaceLong",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeQuoteReplaceLong), gopacket.LayerTypeMetadata{"IttoQuoteReplaceLong", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeQuoteDelete] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoQuoteDelete),
			Name:       "QuoteDelete",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeQuoteDelete), gopacket.LayerTypeMetadata{"IttoQuoteDelete", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeBlockSingleSideDelete] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoBlockSingleSideDelete),
			Name:       "BlockSingleSideDelete",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeBlockSingleSideDelete), gopacket.LayerTypeMetadata{"IttoBlockSingleSideDelete", nil}),
		},
	}
	IttoMessageTypeMetadata[IttoMessageTypeNoii] = EnumMessageTypeMetadata{
		EnumMetadata: layers.EnumMetadata{
			DecodeWith: gopacket.DecodeFunc(decodeIttoNoii),
			Name:       "Noii",
			LayerType:  gopacket.RegisterLayerType(1100+int(IttoMessageTypeNoii), gopacket.LayerTypeMetadata{"IttoNoii", nil}),
		},
	}

	lts := make([]gopacket.LayerType, 0, 256)
	lts = append(lts, LayerTypeIttoUnknown)
	for i := 0; i < 256; i++ {
		lt := IttoMessageTypeMetadata[i].LayerType
		if lt != LayerTypeIttoUnknown {
			lts = append(lts, lt)
		}
	}
	LayerClassItto = gopacket.NewLayerClass(lts)
}

/*
func errorFunc(msg string) gopacket.Decoder {
	var e = errors.New(msg)
	return gopacket.DecodeFunc(func([]byte, gopacket.PacketBuilder) error {
		return e
	})
}
*/

/************************************************************************/
type IttoMessage struct {
	Type      IttoMessageType
	Timestamp uint32
}

func (m *IttoMessage) LayerContents() []byte {
	return nil
}

func (m *IttoMessage) LayerPayload() []byte {
	return nil
}

func decodeIttoMessage(data []byte) IttoMessage {
	m := IttoMessage{
		Type: IttoMessageType(data[0]),
	}
	if m.Type != IttoMessageTypeSeconds && len(data) >= 5 {
		m.Timestamp = binary.BigEndian.Uint32(data[1:5])
	}
	return m
}

func (m *IttoMessage) LayerType() gopacket.LayerType {
	return m.Type.LayerType()
}

/************************************************************************/
var LayerTypeIttoUnknown = gopacket.RegisterLayerType(1100, gopacket.LayerTypeMetadata{"IttoUnknown", nil})

type IttoMessageUnknown struct {
	IttoMessage
	TypeChar string
}

func decodeIttoUnknown(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageUnknown{
		IttoMessage: decodeIttoMessage(data),
		TypeChar:    string(data[0]),
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageSeconds struct {
	IttoMessage
	Second uint32
}

func decodeIttoSeconds(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageSeconds{
		IttoMessage: decodeIttoMessage(data),
		Second:      binary.BigEndian.Uint32(data[1:5]),
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageBaseReference struct {
	IttoMessage
	BaseRefNum uint64
}

func decodeIttoBaseReference(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageBaseReference{
		IttoMessage: decodeIttoMessage(data),
		BaseRefNum:  binary.BigEndian.Uint64(data[5:13]),
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageAddOrder struct {
	IttoMessage
	OId OptionId
	OrderSide
}

func decodeIttoAddOrder(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageAddOrder{
		IttoMessage: decodeIttoMessage(data),
		OrderSide: OrderSide{
			RefNumD: NewRefNumDelta(binary.BigEndian.Uint32(data[5:9])),
			Side:    MarketSideParse(data[9]),
		},
		OId: OptionId(binary.BigEndian.Uint32(data[10:14])),
	}
	if m.Type.IsShort() {
		m.Price = PriceShortToLong(int(binary.BigEndian.Uint16(data[14:16])))
		m.Size = int(binary.BigEndian.Uint16(data[16:18]))
	} else {
		m.Price = int(binary.BigEndian.Uint32(data[14:18]))
		m.Size = int(binary.BigEndian.Uint32(data[18:22]))
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageAddQuote struct {
	IttoMessage
	OId OptionId
	Bid OrderSide
	Ask OrderSide
}

func decodeIttoAddQuote(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageAddQuote{
		IttoMessage: decodeIttoMessage(data),
		Bid:         OrderSide{Side: MarketSideBid, RefNumD: NewRefNumDelta(binary.BigEndian.Uint32(data[5:9]))},
		Ask:         OrderSide{Side: MarketSideAsk, RefNumD: NewRefNumDelta(binary.BigEndian.Uint32(data[9:13]))},
		OId:         OptionId(binary.BigEndian.Uint32(data[13:17])),
	}
	if m.Type.IsShort() {
		m.Bid.Price = PriceShortToLong(int(binary.BigEndian.Uint16(data[17:19])))
		m.Bid.Size = int(binary.BigEndian.Uint16(data[19:21]))
		m.Ask.Price = PriceShortToLong(int(binary.BigEndian.Uint16(data[21:23])))
		m.Ask.Size = int(binary.BigEndian.Uint16(data[23:25]))
	} else {
		m.Bid.Price = int(binary.BigEndian.Uint32(data[17:21]))
		m.Bid.Size = int(binary.BigEndian.Uint32(data[21:25]))
		m.Ask.Price = int(binary.BigEndian.Uint32(data[25:29]))
		m.Ask.Size = int(binary.BigEndian.Uint32(data[29:33]))
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageSingleSideExecuted struct {
	IttoMessage
	OrigRefNumD RefNumDelta
	Size        int
	Cross       uint32
	Match       uint32
}

func decodeIttoSingleSideExecuted(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageSingleSideExecuted{
		IttoMessage: decodeIttoMessage(data),
		OrigRefNumD: OrigRefNumDelta(binary.BigEndian.Uint32(data[5:9])),
		Size:        int(binary.BigEndian.Uint32(data[9:13])),
		Cross:       binary.BigEndian.Uint32(data[13:17]),
		Match:       binary.BigEndian.Uint32(data[17:21]),
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageSingleSideExecutedWithPrice struct {
	IttoMessageSingleSideExecuted
	Printable byte
	Price     int
}

func decodeIttoSingleSideExecutedWithPrice(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageSingleSideExecutedWithPrice{
		IttoMessageSingleSideExecuted: IttoMessageSingleSideExecuted{
			IttoMessage: decodeIttoMessage(data),
			OrigRefNumD: OrigRefNumDelta(binary.BigEndian.Uint32(data[5:9])),
			Cross:       binary.BigEndian.Uint32(data[9:13]),
			Match:       binary.BigEndian.Uint32(data[13:17]),
			Size:        int(binary.BigEndian.Uint32(data[22:26])),
		},
		Printable: data[17],
		Price:     int(binary.BigEndian.Uint32(data[18:22])),
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageOrderCancel struct {
	IttoMessage
	OrigRefNumD RefNumDelta
	Size        int
}

func decodeIttoOrderCancel(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageOrderCancel{
		IttoMessage: decodeIttoMessage(data),
		OrigRefNumD: OrigRefNumDelta(binary.BigEndian.Uint32(data[5:9])),
		Size:        int(binary.BigEndian.Uint32(data[9:13])),
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageSingleSideReplace struct {
	IttoMessage
	ReplaceOrderSide
}

func decodeIttoSingleSideReplace(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageSingleSideReplace{
		IttoMessage: decodeIttoMessage(data),
		ReplaceOrderSide: ReplaceOrderSide{
			OrigRefNumD: OrigRefNumDelta(binary.BigEndian.Uint32(data[5:9])),
			OrderSide:   OrderSide{RefNumD: NewRefNumDelta(binary.BigEndian.Uint32(data[9:13]))},
		},
	}
	if m.Type.IsShort() {
		m.Price = PriceShortToLong(int(binary.BigEndian.Uint16(data[13:15])))
		m.Size = int(binary.BigEndian.Uint16(data[15:17]))
	} else {
		m.Price = int(binary.BigEndian.Uint32(data[13:17]))
		m.Size = int(binary.BigEndian.Uint32(data[17:21]))
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageSingleSideDelete struct {
	IttoMessage
	OrigRefNumD RefNumDelta
}

func decodeIttoSingleSideDelete(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageSingleSideDelete{
		IttoMessage: decodeIttoMessage(data),
		OrigRefNumD: OrigRefNumDelta(binary.BigEndian.Uint32(data[5:9])),
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageSingleSideUpdate struct {
	IttoMessage
	OrderSide
	Reason byte
}

func decodeIttoSingleSideUpdate(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageSingleSideUpdate{
		IttoMessage: decodeIttoMessage(data),
		OrderSide: OrderSide{
			RefNumD: OrigRefNumDelta(binary.BigEndian.Uint32(data[5:9])),
			Price:   int(binary.BigEndian.Uint32(data[10:14])),
			Size:    int(binary.BigEndian.Uint32(data[14:18])),
		},
		Reason: data[9],
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageQuoteReplace struct {
	IttoMessage
	Bid ReplaceOrderSide
	Ask ReplaceOrderSide
}

func decodeIttoQuoteReplace(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageQuoteReplace{
		IttoMessage: decodeIttoMessage(data),
		Bid: ReplaceOrderSide{
			OrigRefNumD: OrigRefNumDelta(binary.BigEndian.Uint32(data[5:9])),
			OrderSide:   OrderSide{Side: MarketSideBid, RefNumD: NewRefNumDelta(binary.BigEndian.Uint32(data[9:13]))},
		},
		Ask: ReplaceOrderSide{
			OrigRefNumD: OrigRefNumDelta(binary.BigEndian.Uint32(data[13:17])),
			OrderSide:   OrderSide{Side: MarketSideAsk, RefNumD: NewRefNumDelta(binary.BigEndian.Uint32(data[17:21]))},
		},
	}
	if m.Type.IsShort() {
		m.Bid.Price = PriceShortToLong(int(binary.BigEndian.Uint16(data[21:23])))
		m.Bid.Size = int(binary.BigEndian.Uint16(data[23:25]))
		m.Ask.Price = PriceShortToLong(int(binary.BigEndian.Uint16(data[25:27])))
		m.Ask.Size = int(binary.BigEndian.Uint16(data[27:29]))
	} else {
		m.Bid.Price = int(binary.BigEndian.Uint32(data[21:25]))
		m.Bid.Size = int(binary.BigEndian.Uint32(data[25:29]))
		m.Ask.Price = int(binary.BigEndian.Uint32(data[29:33]))
		m.Ask.Size = int(binary.BigEndian.Uint32(data[33:37]))
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageQuoteDelete struct {
	IttoMessage
	BidOrigRefNumD RefNumDelta
	AskOrigRefNumD RefNumDelta
}

func decodeIttoQuoteDelete(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageQuoteDelete{
		IttoMessage:    decodeIttoMessage(data),
		BidOrigRefNumD: OrigRefNumDelta(binary.BigEndian.Uint32(data[5:9])),
		AskOrigRefNumD: OrigRefNumDelta(binary.BigEndian.Uint32(data[9:13])),
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageBlockSingleSideDelete struct {
	IttoMessage
	Number   int
	RefNumDs []RefNumDelta
}

func decodeIttoBlockSingleSideDelete(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageBlockSingleSideDelete{
		IttoMessage: decodeIttoMessage(data),
		Number:      int(binary.BigEndian.Uint16(data[5:9])),
	}
	m.RefNumDs = make([]RefNumDelta, m.Number)
	for i := 0; i < m.Number; i++ {
		off := 7 + 4*i
		m.RefNumDs[i] = OrigRefNumDelta(binary.BigEndian.Uint32(data[off : off+4]))
	}
	p.AddLayer(m)
	return nil
}

/************************************************************************/
type IttoMessageNoii struct {
	IttoMessage
	AuctionId   uint32
	AuctionType byte
	Size        uint32
	OId         OptionId
	Imbalance   OrderSide
}

func decodeIttoNoii(data []byte, p gopacket.PacketBuilder) error {
	m := &IttoMessageNoii{
		IttoMessage: decodeIttoMessage(data),
		AuctionId:   binary.BigEndian.Uint32(data[5:9]),
		AuctionType: data[9],
		Size:        binary.BigEndian.Uint32(data[10:14]),
		Imbalance: OrderSide{
			Side:  MarketSideParse(data[14]),
			Price: int(binary.BigEndian.Uint32(data[19:23])),
			Size:  int(binary.BigEndian.Uint32(data[23:27])),
		},
		OId: OptionId(binary.BigEndian.Uint32(data[15:19])),
	}
	p.AddLayer(m)
	return nil
}
