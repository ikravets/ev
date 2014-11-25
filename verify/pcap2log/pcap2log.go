// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package pcap2log

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/kr/pretty"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var textFrameSeparator []byte = []byte("\nFrame ")
var textFrameSeparator1 []byte = []byte("Frame ")

func splitTextFrames(data []byte, atEOF bool) (advance int, token []byte, err error) {
	separatorIndex := bytes.Index(data, textFrameSeparator)
	if separatorIndex == -1 {
		if atEOF {
			log.Println("WARNING skipping before EOF:", string(data))
			return len(data), nil, nil
		} else {
			return 0, nil, nil
		}
	}
	if separatorIndex != 0 {
		if bytes.HasPrefix(data, textFrameSeparator1) {
			separatorIndex = 0
		} else {
			log.Println("WARNING skipping prefix:", string(data[:separatorIndex]))
			return separatorIndex, nil, nil
		}
	}
	// find start of the next frame
	const skip1 = 5
	separatorIndex = bytes.Index(data[skip1:], textFrameSeparator)
	if separatorIndex == -1 {
		if atEOF {
			return len(data), data, nil
		} else {
			return 0, nil, nil
		}
	}
	separatorIndex += skip1

	return separatorIndex, data[:separatorIndex], nil
}

type translator struct {
	r io.Reader
	w io.Writer
	// current message data
	kvStr       map[string]string
	kvInt       map[string]uint
	msgType     byte
	refNumDelta []uint // for Block Single Side Delete Message
	qom         QOMessage
}

func NewTranslator(r io.Reader, w io.Writer) translator {
	return translator{
		r: r,
		w: w,
	}
}

func (t *translator) outMessage1() {
	type ittoOrderInfo struct {
		msgType         byte
		isBid           bool
		isAsk           bool
		optionId        uint
		refNumDelta     uint
		origRefNumDelta uint
		size            uint
		price           uint
	}

	ois := make([]ittoOrderInfo, 0, 3)
	switch t.msgType {
	case 'T', 'I': // ignore Seconds, NOII
	case 'a', 'A':
		ois = append(ois, ittoOrderInfo{
			msgType:     t.msgType,
			refNumDelta: t.kvInt["Order Reference Number Delta"],
			isBid:       byte(t.kvInt["Market Side"]) == 'B',
			isAsk:       byte(t.kvInt["Market Side"]) == 'S',
			optionId:    t.kvInt["Option ID"],
			price:       t.kvInt["Price"],
			size:        t.kvInt["Volume"],
		})
	case 'j', 'J':
		ois = append(ois, ittoOrderInfo{
			msgType:     t.msgType,
			refNumDelta: t.kvInt["Bid Reference Number Delta"],
			optionId:    t.kvInt["Option ID"],
			price:       t.kvInt["Bid Price"],
			size:        t.kvInt["Bid Size"],
			isBid:       true,
		})
		ois = append(ois, ittoOrderInfo{
			msgType:     t.msgType,
			refNumDelta: t.kvInt["Ask Reference Number Delta"],
			optionId:    t.kvInt["Option ID"],
			price:       t.kvInt["Ask Price"],
			size:        t.kvInt["Ask Size"],
			isAsk:       true,
		})
	case 'E':
		ois = append(ois, ittoOrderInfo{
			msgType:     t.msgType,
			refNumDelta: t.kvInt["Reference Number Delta"],
			size:        t.kvInt["Executed Contracts"],
		})
	case 'C':
		ois = append(ois, ittoOrderInfo{
			msgType:     t.msgType,
			refNumDelta: t.kvInt["Reference Number Delta"],
			price:       t.kvInt["Price"],
			size:        t.kvInt["Volume"],
		})
	case 'X':
		ois = append(ois, ittoOrderInfo{
			msgType:     t.msgType,
			refNumDelta: t.kvInt["Order Reference Number Delta"],
			size:        t.kvInt["Cancelled Contracts"],
		})
	case 'u', 'U':
		ois = append(ois, ittoOrderInfo{
			msgType:         t.msgType,
			origRefNumDelta: t.kvInt["Original Reference Number Delta"],
			refNumDelta:     t.kvInt["New Reference Number Delta"],
			price:           t.kvInt["Price"],
			size:            t.kvInt["Volume"],
		})
	case 'D':
		ois = append(ois, ittoOrderInfo{
			msgType:     t.msgType,
			refNumDelta: t.kvInt["Reference Number Delta"],
		})
	case 'G':
		ois = append(ois, ittoOrderInfo{
			msgType:     t.msgType,
			refNumDelta: t.kvInt["Reference Number Delta"],
			price:       t.kvInt["Price"],
			size:        t.kvInt["Volume"],
		})
	case 'k', 'K':
		ois = append(ois, ittoOrderInfo{
			msgType:         t.msgType,
			origRefNumDelta: t.kvInt["Original Bid Reference Number Delta"],
			refNumDelta:     t.kvInt["Bid Reference Number Delta"],
			price:           t.kvInt["Bid Price"],
			size:            t.kvInt["Bid Size"],
			isBid:           true,
		})
		ois = append(ois, ittoOrderInfo{
			msgType:         t.msgType,
			origRefNumDelta: t.kvInt["Original Ask Reference Number Delta"],
			refNumDelta:     t.kvInt["Ask Reference Number Delta"],
			price:           t.kvInt["Ask Price"],
			size:            t.kvInt["Ask Size"],
			isAsk:           true,
		})
	case 'Y':
		ois = append(ois, ittoOrderInfo{
			msgType:     t.msgType,
			refNumDelta: t.kvInt["Bid Reference Number Delta"],
			isBid:       true,
		})
		ois = append(ois, ittoOrderInfo{
			msgType:     t.msgType,
			refNumDelta: t.kvInt["Ask Reference Number Delta"],
			isAsk:       true,
		})

	default:
		log.Fatalf("Unknown message type %d (%c)\n", t.msgType, t.msgType)
	}
	for _, oi := range ois {
		var qo string
		switch {
		case oi.isBid && oi.isAsk:
			log.Fatal("Both bid and ask is set", oi)
		case oi.isBid:
			qo = "QBID"
		case oi.isAsk:
			qo = "QASK"
		default:
			qo = "ORDER"
		}

		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x %08x\n",
			qo,
			oi.msgType,
			oi.optionId,
			oi.refNumDelta,
			oi.origRefNumDelta,
			oi.size,
			oi.price,
		)
	}
}

type MarketSide byte

const (
	MasketSideUnknown MarketSide = 0
	MarketSideBuy                = 'B'
	MarketSideSell               = 'S'
)

type MessageType byte
type OrderSide struct {
	refNumDelta     uint
	origRefNumDelta uint
	price           uint
	size            uint
	side            MarketSide
}
type QOMessage struct {
	typ          MessageType
	timestamp    uint
	optionId     uint
	side1        OrderSide
	side2        OrderSide
	sseCrossNum  uint
	sseMatchNum  uint
	ssePrintable bool
	ssuReason    byte
	bssdNum      uint
	bssdRefs     []uint
}

func (t *translator) translateQOMessage() {
	t.qom = QOMessage{
		typ:       MessageType(t.msgType),
		optionId:  t.kvInt["Option ID"],
		timestamp: t.kvInt["Timestamp"],
	}
	switch t.msgType {
	case 'T', 'L', 'S', 'H', 'O', 'Q', 'I': // ignore Seconds, Base Reference, System,  Options Trading Action, Option Open, Cross Trade, NOII
	case 'j': // Add Quote
		t.qom.side1 = OrderSide{
			side:        MarketSideBuy,
			refNumDelta: t.kvInt["Bid Reference Number Delta"],
			size:        t.kvInt["Bid Size"],
			price:       t.kvInt["Bid Price"],
		}
		t.qom.side2 = OrderSide{
			side:        MarketSideSell,
			refNumDelta: t.kvInt["Ask Reference Number Delta"],
			size:        t.kvInt["Ask Size"],
			price:       t.kvInt["Ask Price"],
		}
	case 'J': // Add Quote
		t.qom.side1 = OrderSide{
			side:        MarketSideBuy,
			refNumDelta: t.kvInt["Bid Reference Number Delta"],
			size:        t.kvInt["Bid Size"],
			price:       t.kvInt["Bid"],
		}
		t.qom.side2 = OrderSide{
			side:        MarketSideSell,
			refNumDelta: t.kvInt["Ask Reference Number Delta"],
			size:        t.kvInt["Ask Size"],
			price:       t.kvInt["Ask"],
		}
	case 'k', 'K': // Quote Replace
		t.qom.side1 = OrderSide{
			side:            MarketSideBuy,
			refNumDelta:     t.kvInt["Bid Reference Number Delta"],
			origRefNumDelta: t.kvInt["Original Bid Reference Number Delta"],
			size:            t.kvInt["Bid Size"],
			price:           t.kvInt["Bid Price"],
		}
		t.qom.side2 = OrderSide{
			side:            MarketSideSell,
			refNumDelta:     t.kvInt["Ask Reference Delta Number"],
			origRefNumDelta: t.kvInt["Original Ask Reference Number Delta"],
			size:            t.kvInt["Ask Size"],
			price:           t.kvInt["Ask Price"],
		}
	case 'Y': // Quote Delete
		t.qom.side1 = OrderSide{
			side:            MarketSideBuy,
			origRefNumDelta: t.kvInt["Bid Reference Number Delta"],
		}
		t.qom.side2 = OrderSide{
			side:            MarketSideSell,
			origRefNumDelta: t.kvInt["Ask Reference Number Delta"],
		}
	case 'a', 'A': // Add Order
		t.qom.side1 = OrderSide{
			side:        MarketSide(t.kvInt["Market Side"]),
			refNumDelta: t.kvInt["Order Reference Number Delta"],
			size:        t.kvInt["Volume"],
			price:       t.kvInt["Price"],
		}
	case 'E': // Single Side Executed
		t.qom.side1 = OrderSide{
			origRefNumDelta: t.kvInt["Reference Number Delta"],
			size:            t.kvInt["Executed Contracts"],
		}
		t.qom.sseCrossNum = t.kvInt["Cross Number"]
		t.qom.sseMatchNum = t.kvInt["Match Number"]
	case 'C': // Single Side Executed with Price
		t.qom.side1 = OrderSide{
			origRefNumDelta: t.kvInt["Reference Number Delta"],
			size:            t.kvInt["Volume"],
			price:           t.kvInt["Price"],
		}
		t.qom.sseCrossNum = t.kvInt["Cross Number"]
		t.qom.sseMatchNum = t.kvInt["Match Number"]
		t.qom.ssePrintable = t.kvStr["Printable"] == "Y"
	case 'X': //  Order Cancel
		t.qom.side1 = OrderSide{
			origRefNumDelta: t.kvInt["Order Reference Number Delta"],
			size:            t.kvInt["Cancelled Contracts"],
		}
	case 'G': // Single Side Update
		t.qom.side1 = OrderSide{
			origRefNumDelta: t.kvInt["Reference Number Delta"],
			price:           t.kvInt["Price"],
			size:            t.kvInt["Volume"],
		}
	case 'u', 'U': // Single Side Replace
		t.qom.side1 = OrderSide{
			refNumDelta:     t.kvInt["New Reference Number Delta"],
			origRefNumDelta: t.kvInt["Original Reference Number Delta"],
			price:           t.kvInt["Price"],
			size:            t.kvInt["Volume"],
		}
	case 'D': // Single Side Delete
		t.qom.side1 = OrderSide{
			origRefNumDelta: t.kvInt["Reference Number Delta"],
		}
	case 'Z': // Block Single Side Delete
		t.qom.bssdNum = t.kvInt["Total Number of Reference Number Deltas."]
		if uint(len(t.refNumDelta)) != t.qom.bssdNum {
			pretty.Println(t.kvInt)
			log.Fatalf("Unexpected number of refs in Z message (%d != %d)\n", t.qom.bssdNum, len(t.refNumDelta))
		}
		t.qom.bssdRefs = append([]uint(nil), t.refNumDelta...)
	default:
		s := pretty.Sprintf("%v", t)
		//log.Fatalf("Unknown message type %d (%c)\n%s\n", t.msgType, t.msgType, s)
		log.Printf("Unknown message type %d (%c)\n%s\n", t.msgType, t.msgType, s)
	}
}

func (t *translator) outMessage3() {
	m := &t.qom
	ord, bid, ask := &m.side1, &m.side1, &m.side2
	if bid.side == MarketSideSell {
		bid, ask = ask, bid
	}
	switch m.typ {
	case 'T', 'L', 'S', 'H', 'O', 'Q', 'I': // ignore Seconds, Base Reference, System,  Options Trading Action, Option Open, Cross Trade, NOII
	case 'j', 'J': // Add Quote
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", m.typ, m.optionId, bid.refNumDelta, bid.size, bid.price)
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", m.typ, m.optionId, ask.refNumDelta, ask.size, ask.price)
	case 'k', 'K': // Quote Replace
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", m.typ, bid.refNumDelta, bid.origRefNumDelta, bid.size, bid.price)
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", m.typ, ask.refNumDelta, ask.origRefNumDelta, ask.size, ask.price)
	case 'Y': // Quote Delete
		fmt.Fprintf(t.w, "%s %c %08x\n",
			"NORM QBID", m.typ, bid.origRefNumDelta)
		fmt.Fprintf(t.w, "%s %c %08x\n",
			"NORM QASK", m.typ, ask.origRefNumDelta)
	case 'a', 'A': // Add Order
		fmt.Fprintf(t.w, "%s %c %c %08x %08x %08x %08x\n",
			"NORM ORDER", m.typ, ord.side, m.optionId, ord.refNumDelta, ord.size, ord.price)
	case 'E', 'C': // Single Side Executed (with Price)
		fmt.Fprintf(t.w, "%s %c %08x %08x\n",
			"NORM ORDER", m.typ, ord.origRefNumDelta, ord.size)
	case 'X': //  Order Cancel
		fmt.Fprintf(t.w, "%s %c %08x %08x\n",
			"NORM ORDER", m.typ, ord.origRefNumDelta, ord.size)
	case 'G': // Single Side Update
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x\n",
			"NORM ORDER", m.typ, ord.origRefNumDelta, ord.size, ord.price)
	case 'u', 'U': // Single Side Replace
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM ORDER", m.typ, ord.refNumDelta, ord.origRefNumDelta, ord.size, ord.price)
	case 'D': // Single Side Delete
		fmt.Fprintf(t.w, "%s %c %08x\n",
			"NORM ORDER", m.typ, ord.origRefNumDelta)
	case 'Z': // Block Single Side Delete
		for _, r := range m.bssdRefs {
			fmt.Fprintf(t.w, "%s %c %08x\n", "NORM ORDER", m.typ, r)
		}
	default:
		s := pretty.Sprintf("%v", t)
		//log.Fatalf("Unknown message type %d (%c)\n%s\n", m.typ, m.typ, s)
		log.Printf("Unknown message type %d (%c)\n%s\n", m.typ, m.typ, s)
	}
}

func (t *translator) outMessage2() {
	switch t.msgType {
	case 'T', 'L', 'S', 'H', 'O', 'Q', 'I': // ignore Seconds, Base Reference, System,  Options Trading Action, Option Open, Cross Trade, NOII
	case 'j': // Add Quote
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", t.msgType,
			t.kvInt["Option ID"],
			t.kvInt["Bid Reference Number Delta"],
			t.kvInt["Bid Size"],
			t.kvInt["Bid Price"],
		)
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", t.msgType,
			t.kvInt["Option ID"],
			t.kvInt["Ask Reference Number Delta"],
			t.kvInt["Ask Size"],
			t.kvInt["Ask Price"],
		)
	case 'J': // Add Quote
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", t.msgType,
			t.kvInt["Option ID"],
			t.kvInt["Bid Reference Number Delta"],
			t.kvInt["Bid Size"],
			t.kvInt["Bid"],
		)
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", t.msgType,
			t.kvInt["Option ID"],
			t.kvInt["Ask Reference Number Delta"],
			t.kvInt["Ask Size"],
			t.kvInt["Ask"],
		)
	case 'k', 'K': // Quote Replace
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", t.msgType,
			t.kvInt["Bid Reference Number Delta"],
			t.kvInt["Original Bid Reference Number Delta"],
			t.kvInt["Bid Size"],
			t.kvInt["Bid Price"],
		)
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", t.msgType,
			t.kvInt["Ask Reference Delta Number"],
			t.kvInt["Original Ask Reference Number Delta"],
			t.kvInt["Ask Size"],
			t.kvInt["Ask Price"],
		)
	case 'Y': // Quote Delete
		fmt.Fprintf(t.w, "%s %c %08x\n",
			"NORM QBID", t.msgType,
			t.kvInt["Bid Reference Number Delta"],
		)
		fmt.Fprintf(t.w, "%s %c %08x\n",
			"NORM QASK", t.msgType,
			t.kvInt["Ask Reference Number Delta"],
		)
	case 'a', 'A': // Add Order
		fmt.Fprintf(t.w, "%s %c %c %08x %08x %08x %08x\n",
			"NORM ORDER", t.msgType,
			t.kvInt["Market Side"],
			t.kvInt["Option ID"],
			t.kvInt["Order Reference Number Delta"],
			t.kvInt["Volume"],
			t.kvInt["Price"],
		)
	case 'E': // Single Side Executed
		fmt.Fprintf(t.w, "%s %c %08x %08x\n",
			"NORM ORDER", t.msgType,
			t.kvInt["Reference Number Delta"],
			t.kvInt["Executed Contracts"],
		)
	case 'C': // Single Side Executed with Price
		fmt.Fprintf(t.w, "%s %c %08x %08x\n",
			"NORM ORDER", t.msgType,
			t.kvInt["Reference Number Delta"],
			t.kvInt["Volume"],
		)
	case 'X': //  Order Cancel
		fmt.Fprintf(t.w, "%s %c %08x %08x\n",
			"NORM ORDER", t.msgType,
			t.kvInt["Order Reference Number Delta"],
			t.kvInt["Cancelled Contracts"],
		)
	case 'G': // Single Side Update
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x\n",
			"NORM ORDER", t.msgType,
			t.kvInt["Reference Number Delta"],
			t.kvInt["Volume"],
			t.kvInt["Price"],
		)
	case 'u', 'U': // Single Side Replace
		fmt.Fprintf(t.w, "%s %c %08x %08x %08x %08x\n",
			"NORM ORDER", t.msgType,
			t.kvInt["New Reference Number Delta"],
			t.kvInt["Original Reference Number Delta"],
			t.kvInt["Volume"],
			t.kvInt["Price"],
		)
	case 'D': // Single Side Delete
		fmt.Fprintf(t.w, "%s %c %08x\n",
			"NORM ORDER", t.msgType,
			t.kvInt["Reference Number Delta"],
		)
	case 'Z': // Block Single Side Delete
		exp := t.kvInt["Total Number of Reference Number Deltas."]
		if uint(len(t.refNumDelta)) != exp {
			pretty.Println(t.kvInt)
			log.Fatalf("Unexpected number of refs in Z message (%d != %d)\n", exp, len(t.refNumDelta))
		}
		for _, n := range t.refNumDelta {
			fmt.Fprintf(t.w, "%s %c %08x\n",
				"NORM ORDER", t.msgType,
				n,
			)
		}
	default:
		s := pretty.Sprintf("%v", t)
		//log.Fatalf("Unknown message type %d (%c)\n%s\n", t.msgType, t.msgType, s)
		log.Printf("Unknown message type %d (%c)\n%s\n", t.msgType, t.msgType, s)
	}
}

func (t *translator) translate() {
	kvRegexp := regexp.MustCompile("(?m)^            ([^:]*): (.*)$")
	parValueRegexp := regexp.MustCompile(".*\\((\\d+)\\)")
	scanner := bufio.NewScanner(t.r)
	scanner.Split(splitTextFrames)
	for scanner.Scan() {
		//fmt.Println("=====================")
		//fmt.Println(scanner.Text())
		ittoMessages := strings.Split(scanner.Text(), "        ITTO ")
		if len(ittoMessages) == 1 {
			continue
		}
		for _, ittoMessage := range ittoMessages[1:] {
			matches := kvRegexp.FindAllStringSubmatch(ittoMessage, -1)
			t.kvStr = make(map[string]string)
			t.kvInt = make(map[string]uint)
			t.refNumDelta = nil
			t.msgType = 0
			for _, m := range matches {
				k := m[1]
				v := m[2]
				if t.msgType == 'Z' && k == "Reference Number Delta" {
					vInt, err := strconv.ParseUint(v, 0, 32)
					if err != nil {
						log.Fatal("Can't parse", v)
					}
					t.refNumDelta = append(t.refNumDelta, uint(vInt))
				} else {
					if _, ok := t.kvStr[k]; ok {
						pretty.Println(ittoMessage)
						pretty.Println(matches)
						pretty.Println(m)
						log.Fatal("Duplicate key ", k)
					}
					t.kvStr[k] = v
					vInt, err := strconv.ParseUint(v, 0, 32)
					if err == nil {
						t.kvInt[k] = uint(vInt)
					} else if matches := parValueRegexp.FindStringSubmatch(v); matches != nil {
						vInt, err := strconv.ParseUint(matches[1], 0, 32)
						t.kvInt[k] = uint(vInt)
						if err != nil {
							log.Fatal("Can't parse", v)
						}
						if k == "Message Type" {
							t.msgType = byte(vInt)
						}
					}
				}
			}
			//pretty.Println(t.kvStr)
			//pretty.Println(t.kvInt)
			//t.outMessage2()
			t.translateQOMessage()
			t.outMessage3()
		}
	}
}

func getTsharkDump(fileName string, args []string) (reader io.Reader, finisher func()) {
	//pretty.Println(fileName, args)
	cmdArgs := []string{
		"-d", "udp.port==18000:10,moldudp64",
		"-V",
		"-r",
		fileName,
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("tshark", cmdArgs...)
	reader, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	finisher = func() {
		if err := cmd.Wait(); err != nil {
			log.Fatal(err)
		}
	}
	return
}

type pcap2log struct {
	InputFileName  string                        `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	OutputFileName string                        `long:"output" short:"o" value-name:"FILE" default:"/dev/stdout" default-mask:"stdout" description:"output file"`
	Args           struct{ TsharkArgs []string } `positional-args:"y"`
}

func (p *pcap2log) Execute(args []string) error {
	//fmt.Println("pcap2log Executed", p, args)
	//pretty.Println(p)
	//pretty.Println(args)
	dumpReader, finisher := getTsharkDump(p.InputFileName, p.Args.TsharkArgs)
	defer finisher()
	outFile, err := os.OpenFile(p.OutputFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()
	t := NewTranslator(dumpReader, outFile)
	t.translate()
	return nil
}

func InitArgv(parser *flags.Parser) {
	var p2l pcap2log
	parser.AddCommand("pcap2log",
		"convert pcap file to simulator output",
		"",
		&p2l)
}

/*****************************************************************************/
// experiments and debugging

func main() {
	t := NewTranslator(os.Stdin, os.Stdout)
	t.translate()
	_ = pretty.Print
	_ = fmt.Print

}
