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
	"syscall"
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
	r   io.Reader
	w   io.Writer
	sim simulator
	// current message data
	kvStr       map[string]string
	kvInt       map[string]uint
	msgType     byte
	refNumDelta []uint // for Block Single Side Delete Message
	qom         QOMessage
}

func NewTranslator(r io.Reader, w io.Writer) translator {
	return translator{
		r:   r,
		w:   w,
		sim: NewSimulator(w),
	}
}

type MarketSide byte

const (
	MarketSideUnknown MarketSide = 0
	MarketSideBuy                = 'B'
	MarketSideSell               = 'S'
)

type MessageType byte

const (
	MessageTypeUnknown MessageType = iota
	MessageTypeQuoteAdd
	MessageTypeQuoteReplace
	MessageTypeQuoteDelete
	MessageTypeOrderAdd
	MessageTypeOrderExecute
	MessageTypeOrderExecuteWPrice
	MessageTypeOrderCancel
	MessageTypeOrderUpdate
	MessageTypeOrderReplace
	MessageTypeOrderDelete
	MessageTypeBlockOrderDelete
)

type OptionId uint

const OptionIdUnknown OptionId = 0

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
	optionId     OptionId
	side1        OrderSide
	side2        OrderSide
	sseCrossNum  uint
	sseMatchNum  uint
	ssePrintable bool
	ssuReason    byte
	bssdNum      uint
	bssdRefs     []uint
}

var charToMessageType = []MessageType{
	'j': MessageTypeQuoteAdd,
	'J': MessageTypeQuoteAdd,
	'k': MessageTypeQuoteReplace,
	'K': MessageTypeQuoteReplace,
	'Y': MessageTypeQuoteDelete,
	'a': MessageTypeOrderAdd,
	'A': MessageTypeOrderAdd,
	'E': MessageTypeOrderExecute,
	'C': MessageTypeOrderExecuteWPrice,
	'X': MessageTypeOrderCancel,
	'G': MessageTypeOrderUpdate,
	'u': MessageTypeOrderReplace,
	'U': MessageTypeOrderReplace,
	'D': MessageTypeOrderDelete,
	'Z': MessageTypeBlockOrderDelete,
}

func (t *translator) translateQOMessage() {
	t.qom = QOMessage{
		typ:       charToMessageType[t.msgType],
		timestamp: t.kvInt["Timestamp"],
	}
	if oid := t.kvInt["Option ID"]; oid != 0 {
		t.qom.optionId = OptionId(oid)
	} else {
		t.qom.optionId = OptionIdUnknown
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

func (t *translator) Translate() {
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
			// TODO be more gc friendly
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
			t.translateQOMessage()
			t.sim.addMessage(&t.qom, t.msgType)
		}
	}
	t.sim.logStats()
}

func startCommandPipe(name string, args []string, ignoreSigpipe bool) (pipe io.ReadCloser, finisher func()) {
	cmd := exec.Command(name, args...)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	finisher = func() {
		pipe.Close()
		for err := cmd.Wait(); err != nil; {
			if ignoreSigpipe {
				if exiterr, ok := err.(*exec.ExitError); ok {
					if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
						if status.Signaled() && status.Signal() == syscall.SIGPIPE {
							log.Printf("WARNING: command %s exited with SIGPIPE\n", name)
							break
						}
					}
				}
			}
			log.Fatal(err)
		}
	}
	return
}

func getTsharkDump(fileName string, args []string) (reader io.Reader, finisher func()) {
	cmdArgs := []string{
		"-d", "udp.port==18000:10,moldudp64",
		"-V",
		"-r",
		fileName,
	}
	cmdArgs = append(cmdArgs, args...)
	reader, finisher = startCommandPipe("tshark", cmdArgs, false)
	return
}

type pcap2log struct {
	InputFileName  string                        `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	OutputFileName string                        `long:"output" short:"o" value-name:"FILE" default:"/dev/stdout" default-mask:"stdout" description:"output file"`
	Args           struct{ TsharkArgs []string } `positional-args:"y"`
	shouldExecute  bool
}

func (p *pcap2log) Execute(args []string) error {
	p.shouldExecute = true
	return nil
}

func (p *pcap2log) maybeRun() error {
	if !p.shouldExecute {
		return nil
	}
	dumpReader, finisher := getTsharkDump(p.InputFileName, p.Args.TsharkArgs)
	defer finisher()
	outFile, err := os.OpenFile(p.OutputFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()
	t := NewTranslator(dumpReader, outFile)
	t.Translate()
	return nil
}

func InitArgv(parser *flags.Parser) func() error {
	var command pcap2log
	parser.AddCommand("pcap2log",
		"convert pcap file to simulator output",
		"",
		&command)
	return command.maybeRun
}

/*****************************************************************************/
// experiments and debugging

func main() {
	t := NewTranslator(os.Stdin, os.Stdout)
	t.Translate()
	_ = pretty.Print
	_ = fmt.Print

}