// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"encoding/csv"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/ikravets/errs"
	"github.com/jessevdk/go-flags"
)

type cmdCleanavt struct {
	InputFileTicker string `long:"input" short:"i" value-name:"FILE" default:"/dev/stdin" default-mask:"stdin" description:"input ticker file"`
	InputFileDict   string `long:"dict" short:"d" required:"y" value-name:"DICT" description:"dictionary for AVT CSV output"`
	OutputFileName  string `long:"output" short:"o" value-name:"FILE" default:"/dev/stdout" default-mask:"stdout" description:"output file"`
	shouldExecute   bool
}

func (c *cmdCleanavt) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdCleanavt) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("cleanavt", "clean AVT CSV ticker file", "", c)
}

func (c *cmdCleanavt) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	inFile, err := os.OpenFile(c.InputFileTicker, os.O_RDONLY, 0644)
	errs.CheckE(err)
	defer inFile.Close()
	dictFile, err := os.OpenFile(c.InputFileDict, os.O_RDONLY, 0644)
	errs.CheckE(err)
	defer dictFile.Close()
	outFile, err := os.OpenFile(c.OutputFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	errs.CheckE(err)
	defer outFile.Close()

	dict := loadDictReverse(dictFile)
	errs.CheckE(filterAvt(inFile, outFile, dict))
}

func init() {
	var c cmdCleanavt
	Registry.Register(&c)
}

func loadDictReverse(rDict io.Reader) map[string]int {
	r := csv.NewReader(rDict)
	records, err := r.ReadAll()
	errs.CheckE(err)
	avtName2Oid := make(map[string]int)
	for _, rec := range records {
		errs.Check(len(rec) == 2, "bad csv record", rec)
		if rec[0] == "avtSymbol" && rec[1] == "exchangeId" {
			continue // header
		}
		name := rec[0]
		oid, err := strconv.Atoi(rec[1])
		errs.CheckE(err, rec)
		avtName2Oid[name] = oid
	}
	return avtName2Oid
}

type avtCsvRecord struct {
	OptionMarketDataNASDAQ2 string
	Date                    string
	Time                    string
	Security                string
	Underlying              string
	SecurityType            string
	BidSize                 string
	BidPrice                string
	OrderBidSize            string
	AskSize                 string
	AskPrice                string
	OrderAskSize            string
	TradeStatus             string
	TickCondition           string
	ExchangeTimestamp       string
}

func avtCsvRecordFromStrings(ss []string) avtCsvRecord {
	a := avtCsvRecord{
		OptionMarketDataNASDAQ2: ss[0],
		Date:              ss[1],
		Time:              ss[2],
		Security:          ss[3],
		Underlying:        ss[4],
		SecurityType:      ss[5],
		BidSize:           ss[6],
		BidPrice:          ss[7],
		OrderBidSize:      ss[8],
		AskSize:           ss[9],
		AskPrice:          ss[10],
		OrderAskSize:      ss[11],
		TradeStatus:       ss[12],
		TickCondition:     ss[13],
		ExchangeTimestamp: ss[14],
	}
	return a
}

func filterAvt(rCsv io.Reader, wCsv io.Writer, dict map[string]int) (err error) {
	r := csv.NewReader(rCsv)
	w := csv.NewWriter(wCsv)
	badNames := make(map[string]struct{})
	location, err := time.LoadLocation("EST")
	errs.CheckE(err)
	for {
		var recIn, recOut []string
		if recIn, err = r.Read(); err != nil {
			//log.Printf("csv error %s, rec %#v\n", err, recIn)
			if err == io.EOF {
				break
			} else if perr := err.(*csv.ParseError); perr != nil && perr.Err == csv.ErrFieldCount && recIn[0] == "end of stream" {
				break
			}
			return
		}
		if recIn[1] == "date" && recIn[2] == "time" {
			continue // header
		}
		a := avtCsvRecordFromStrings(recIn)

		// validate security name
		if _, ok := dict[a.Security]; !ok {
			if _, ok := badNames[a.Security]; !ok {
				// report only the first time
				//log.Printf("oid not found for name %s\n", a.Security)
				badNames[a.Security] = struct{}{}
			}
			continue
		}

		// validate non-empty book
		bidSize, _ := strconv.Atoi(a.BidSize)
		askSize, _ := strconv.Atoi(a.AskSize)
		bidPrice, _ := strconv.ParseFloat(a.BidPrice, 64)
		askPrice, _ := strconv.ParseFloat(a.AskPrice, 64)
		if (bidSize == 0 || bidPrice == 0) && (askSize == 0 || askPrice == 0) {
			continue
		}

		errs.Check(len(a.Time) > 4, "time field too short", a.Time)
		a.Time = a.Time[:len(a.Time)-4]
		// validate sane timestamp
		var appTime time.Time
		appTime, err = time.ParseInLocation("2006010215:04:05.000", a.Date+a.Time, location)
		if err != nil {
			return
		}
		var excTimestamp int
		excTimestamp, err = strconv.Atoi(a.ExchangeTimestamp)
		if err != nil {
			return
		}
		diff := appTime.Unix() - int64(excTimestamp)/1000
		if diff > 1000 || diff < -300 {
			continue
		}

		recOut = []string{
			a.ExchangeTimestamp,
			a.Date,
			a.Time,
			a.Security,
			a.BidSize,
			a.BidPrice,
			a.AskSize,
			a.AskPrice,
		}
		errs.CheckE(w.Write(recOut))
	}
	w.Flush()
	return w.Error()
}
