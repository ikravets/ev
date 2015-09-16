// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package sbtcp

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/ikravets/errs"
)

type MessageType byte

type Message interface {
	getCommon() *MessageCommon
	getHeader() MessageHeader
	decodePayload() error
	encodePayload() error
	Type() MessageType
	SetPayload([]byte)
}

const (
	TypeDebug         MessageType = '+'
	TypeLoginAccepted MessageType = 'A'
	TypeLoginRejected MessageType = 'J'
	TypeSequencedData MessageType = 'S'
	TypeHeartbeat     MessageType = 'H'
	TypeEnd           MessageType = 'Z'

	TypeLoginRequest    MessageType = 'L'
	TypeUnsequencedData MessageType = 'U'
	TypeClientHeartbeat MessageType = 'R'
	TypeLogout          MessageType = 'O'
)

type MessageHeader struct {
	Length uint16
	Type   MessageType
}

type MessageCommon struct {
	Header  MessageHeader
	Payload []byte
}

func (mc *MessageCommon) getCommon() *MessageCommon {
	return mc
}
func (mc *MessageCommon) getHeader() MessageHeader {
	return mc.Header
}
func (m *MessageCommon) decodePayload() (err error) {
	return
}
func (m *MessageCommon) encodePayload() (err error) {
	return
}
func (mc *MessageCommon) Type() MessageType {
	return mc.Header.Type
}
func (mc *MessageCommon) SetPayload(data []byte) {
	mc.Payload = data
}
func (mc *MessageCommon) setHeader(Type MessageType) error {
	var payloadSize int
	switch Type {
	case TypeDebug, TypeSequencedData, TypeUnsequencedData:
		payloadSize = len(mc.Payload)
	case TypeLoginAccepted:
		payloadSize = 30
	case TypeLoginRejected:
		payloadSize = 1
	case TypeLoginRequest:
		payloadSize = 46
	}
	errs.Check(payloadSize == len(mc.Payload), payloadSize, len(mc.Payload))
	errs.Check(payloadSize < math.MaxUint16)
	mc.Header.Type = Type
	mc.Header.Length = uint16(payloadSize) + 1
	return nil
}

type MessageDebug struct {
	MessageCommon
}

type MessageLoginAccepted struct {
	MessageCommon
	Session        string
	SequenceNumber int
}

func (m *MessageLoginAccepted) decodePayload() (err error) {
	m.Session = string(m.Payload[0:10])
	m.SequenceNumber, err = strconv.Atoi(strings.TrimSpace(string(m.Payload[10:30])))
	return
}
func (m *MessageLoginAccepted) encodePayload() (err error) {
	m.Payload = make([]byte, 30)
	copy(m.Payload[0:10], []byte(m.Session))
	errs.Check(m.SequenceNumber > 0)
	copy(m.Payload[10:30], []byte(fmt.Sprintf("%020d", m.SequenceNumber)))
	m.Header.Type = TypeLoginAccepted
	m.Header.Length = uint16(len(m.Payload) + 1)
	return nil
}

const (
	LoginRejectedUnauthorized = 'A'
	LoginRejectedBadSession   = 'S'
)

type MessageLoginRejected struct {
	MessageCommon
	Reason byte
}

func (m *MessageLoginRejected) decodePayload() (err error) {
	m.Reason = m.Payload[0]
	return
}

type MessageSequencedData struct {
	MessageCommon
}

type MessageHeartbeat struct {
	MessageCommon
}

type MessageEnd struct {
	MessageCommon
}

type MessageLoginRequest struct {
	MessageCommon
	Username       string
	Password       string
	Session        string
	SequenceNumber int
}

func (m *MessageLoginRequest) decodePayload() (err error) {
	m.Username = string(m.Payload[0:6])
	m.Password = string(m.Payload[6:16])
	m.Session = string(m.Payload[16:26])
	m.SequenceNumber, err = strconv.Atoi(strings.TrimSpace(string(m.Payload[26:46])))
	return
}

type MessageUnsequencedData struct {
	MessageCommon
}

type MessageClientHeartbeat struct {
	MessageCommon
}

type MessageLogout struct {
	MessageCommon
}

func ReadMessage(r io.Reader) (m Message, err error) {
	defer errs.PassE(&err)
	var mc MessageCommon
	errs.CheckE(binary.Read(r, binary.BigEndian, &mc.Header))
	mc.Payload = make([]byte, mc.Header.Length-1)
	n, err := r.Read(mc.Payload)
	errs.CheckE(err)
	errs.Check(n == len(mc.Payload), n, len(mc.Payload))
	switch mc.Header.Type {
	case TypeDebug:
		m = &MessageDebug{}
	case TypeLoginAccepted:
		m = &MessageLoginAccepted{}
	case TypeLoginRejected:
		m = &MessageLoginRejected{}
	case TypeSequencedData:
		m = &MessageSequencedData{}
	case TypeHeartbeat:
		m = &MessageHeartbeat{}
	case TypeEnd:
		m = &MessageEnd{}
	case TypeLoginRequest:
		m = &MessageLoginRequest{}
	case TypeUnsequencedData:
		m = &MessageUnsequencedData{}
	case TypeClientHeartbeat:
		m = &MessageClientHeartbeat{}
	case TypeLogout:
		m = &MessageLogout{}
	}
	*m.getCommon() = mc
	errs.CheckE(m.decodePayload())
	return
}

func WriteMessage(w io.Writer, m Message) (err error) {
	defer errs.PassE(&err)
	errs.CheckE(m.encodePayload())
	var mt MessageType
	switch m.(type) {
	case *MessageDebug:
		mt = TypeDebug
	case *MessageLoginAccepted:
		mt = TypeLoginAccepted
	case *MessageLoginRejected:
		mt = TypeLoginRejected
	case *MessageSequencedData:
		mt = TypeSequencedData
	case *MessageHeartbeat:
		mt = TypeHeartbeat
	case *MessageEnd:
		mt = TypeEnd
	case *MessageLoginRequest:
		mt = TypeLoginRequest
	case *MessageUnsequencedData:
		mt = TypeUnsequencedData
	case *MessageClientHeartbeat:
		mt = TypeClientHeartbeat
	case *MessageLogout:
		mt = TypeLogout
	}
	errs.CheckE(m.getCommon().setHeader(mt))
	errs.CheckE(binary.Write(w, binary.BigEndian, m.getHeader()))
	n, err := w.Write(m.getCommon().Payload)
	errs.CheckE(err)
	errs.Check(n == len(m.getCommon().Payload))
	return
}
