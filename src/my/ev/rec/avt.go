// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	"my/ev/sim"
)

var _ sim.Observer = &AvtLogger{}

type AvtLogger struct {
	w           io.Writer
	location    *time.Location
	oid2AvtName map[int]string
	TobLogger
	stream Stream
}

func NewAvtLogger(w io.Writer, rDict io.Reader) *AvtLogger {
	l := &AvtLogger{
		w:         w,
		TobLogger: *NewTobLogger(),
		stream:    *NewStream(),
	}
	var err error
	if l.location, err = time.LoadLocation("EST"); err != nil {
		log.Fatal(err)
	}

	if rDict != nil {
		r := csv.NewReader(rDict)
		records, err := r.ReadAll()
		if err != nil {
			log.Fatal(err)
		}
		l.oid2AvtName = make(map[int]string)
		for _, rec := range records {
			if len(rec) != 2 {
				log.Fatalf("unexpected dict csv record: %#v\n", rec)
			}
			if rec[0] == "avtSymbol" && rec[1] == "exchangeId" {
				continue // header
			}
			if oid, err := strconv.Atoi(rec[1]); err != nil {
				log.Fatal(err)
			} else {
				l.oid2AvtName[oid] = rec[0]
			}
		}
	}
	return l
}

func (l *AvtLogger) MessageArrived(idm *sim.SimMessage) {
	l.stream.MessageArrived(idm)
	l.TobLogger.MessageArrived(idm)
}

func (l *AvtLogger) AfterBookUpdate(book sim.Book, operation sim.SimOperation) {
	if l.TobLogger.AfterBookUpdate(book, operation, TobUpdateNewForce) {
		l.genUpdate()
	}
}

func (l *AvtLogger) genUpdate() {
	var optName, underlying string
	if l.oid2AvtName != nil {
		optName = l.oid2AvtName[int(l.lastOptionId.ToUint32())]
		bs := []byte(optName)
		for i := range bs {
			if bs[i] < 'A' || bs[i] > 'Z' {
				bs = bs[:i]
				break
			}
		}
		underlying = string(bs)

	}
	if optName == "" {
		optName = fmt.Sprintf("<%d>", l.lastOptionId)
		underlying = "<?>"
	}
	packetTimestamp := l.stream.getPacketTimestamp().In(l.location)
	dateTime := packetTimestamp.Format("20060102,15:04:05.000.")
	dateTime += fmt.Sprintf("%03d", packetTimestamp.Nanosecond()/1000%1000)
	dayStart := time.Date(packetTimestamp.Year(), packetTimestamp.Month(), packetTimestamp.Day(), 0, 0, 0, 0, l.location)
	avtTimestamp := dayStart.Add(time.Duration(l.stream.getTimestamp())).UnixNano() / 1000000
	if false {
		// OptionMarketDataNASDAQ2,date,time,Security,Underlying,SecurityType,BidSize,BidPrice,OrderBidSize,AskSize,AskPrice,OrderAskSize,TradeStatus,TickCondition,ExchangeTimestamp
		fmt.Fprintf(l.w, "OptionMarketDataNASDAQ2,%s,%s,%s,1,%d,%s,0,%d,%s,0,,,%d\n",
			dateTime,
			optName,
			underlying,
			l.bid.New.Size,
			priceString(l.bid.New.Price),
			l.ask.New.Size,
			priceString(l.ask.New.Price),
			avtTimestamp,
		)
	} else {
		if underlying == "<?>" {
			return
		}
		// ExchangeTimestamp, Date, Time, Security, BidSize, BidPrice, AskSize, AskPrice,
		fmt.Fprintf(l.w, "%d,%s,%s,%d,%s,%d,%s\n",
			avtTimestamp,
			dateTime[0:21],
			optName,
			l.bid.New.Size,
			priceString(l.bid.New.Price),
			l.ask.New.Size,
			priceString(l.ask.New.Price),
		)
	}
}

func priceString(price int) string {
	var buf bytes.Buffer
	buf.Write(strconv.AppendInt(nil, int64(price/10000), 10))
	buf.WriteByte('.')
	decimal := price % 10000
	for denom := 1000; denom > 0; denom /= 10 {
		d := decimal / denom % 10
		buf.WriteByte('0' + byte(d))
		if decimal%denom == 0 {
			break
		}
	}
	return buf.String()
}
