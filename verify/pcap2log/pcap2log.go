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

func outMessage1(w io.Writer, kvStr map[string]string, kvInt map[string]uint) {
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
	msgType := byte(kvInt["Message Type"])
	switch msgType {
	case 'T', 'I': // ignore Seconds, NOII
	case 'a', 'A':
		ois = append(ois, ittoOrderInfo{
			msgType:     msgType,
			refNumDelta: kvInt["Order Reference Number Delta"],
			isBid:       byte(kvInt["Market Side"]) == 'B',
			isAsk:       byte(kvInt["Market Side"]) == 'S',
			optionId:    kvInt["Option ID"],
			price:       kvInt["Price"],
			size:        kvInt["Volume"],
		})
	case 'j', 'J':
		ois = append(ois, ittoOrderInfo{
			msgType:     msgType,
			refNumDelta: kvInt["Bid Reference Number Delta"],
			optionId:    kvInt["Option ID"],
			price:       kvInt["Bid Price"],
			size:        kvInt["Bid Size"],
			isBid:       true,
		})
		ois = append(ois, ittoOrderInfo{
			msgType:     msgType,
			refNumDelta: kvInt["Ask Reference Number Delta"],
			optionId:    kvInt["Option ID"],
			price:       kvInt["Ask Price"],
			size:        kvInt["Ask Size"],
			isAsk:       true,
		})
	case 'E':
		ois = append(ois, ittoOrderInfo{
			msgType:     msgType,
			refNumDelta: kvInt["Reference Number Delta"],
			size:        kvInt["Executed Contracts"],
		})
	case 'C':
		ois = append(ois, ittoOrderInfo{
			msgType:     msgType,
			refNumDelta: kvInt["Reference Number Delta"],
			price:       kvInt["Price"],
			size:        kvInt["Volume"],
		})
	case 'X':
		ois = append(ois, ittoOrderInfo{
			msgType:     msgType,
			refNumDelta: kvInt["Order Reference Number Delta"],
			size:        kvInt["Cancelled Contracts"],
		})
	case 'u', 'U':
		ois = append(ois, ittoOrderInfo{
			msgType:         msgType,
			origRefNumDelta: kvInt["Original Reference Number Delta"],
			refNumDelta:     kvInt["New Reference Number Delta"],
			price:           kvInt["Price"],
			size:            kvInt["Volume"],
		})
	case 'D':
		ois = append(ois, ittoOrderInfo{
			msgType:     msgType,
			refNumDelta: kvInt["Reference Number Delta"],
		})
	case 'G':
		ois = append(ois, ittoOrderInfo{
			msgType:     msgType,
			refNumDelta: kvInt["Reference Number Delta"],
			price:       kvInt["Price"],
			size:        kvInt["Volume"],
		})
	case 'k', 'K':
		ois = append(ois, ittoOrderInfo{
			msgType:         msgType,
			origRefNumDelta: kvInt["Original Bid Reference Number Delta"],
			refNumDelta:     kvInt["Bid Reference Number Delta"],
			price:           kvInt["Bid Price"],
			size:            kvInt["Bid Size"],
			isBid:           true,
		})
		ois = append(ois, ittoOrderInfo{
			msgType:         msgType,
			origRefNumDelta: kvInt["Original Ask Reference Number Delta"],
			refNumDelta:     kvInt["Ask Reference Number Delta"],
			price:           kvInt["Ask Price"],
			size:            kvInt["Ask Size"],
			isAsk:           true,
		})
	case 'Y':
		ois = append(ois, ittoOrderInfo{
			msgType:     msgType,
			refNumDelta: kvInt["Bid Reference Number Delta"],
			isBid:       true,
		})
		ois = append(ois, ittoOrderInfo{
			msgType:     msgType,
			refNumDelta: kvInt["Ask Reference Number Delta"],
			isAsk:       true,
		})

	default:
		log.Fatalf("Unknown message type %d (%c)\n", msgType, msgType)
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

		fmt.Fprintf(w, "%s %c %08x %08x %08x %08x %08x\n",
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

func outMessage2(w io.Writer, kvStr map[string]string, kvInt map[string]uint) {
	msgType := byte(kvInt["Message Type"])
	switch msgType {
	case 'T', 'L', 'I': // ignore Seconds, NOII
	case 'j', 'J': // Add Quote
		fmt.Fprintf(w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", msgType,
			kvInt["Option ID"],
			kvInt["Bid Reference Number Delta"],
			kvInt["Bid Size"],
			kvInt["Bid Price"],
		)
		fmt.Fprintf(w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", msgType,
			kvInt["Option ID"],
			kvInt["Ask Reference Number Delta"],
			kvInt["Ask Size"],
			kvInt["Ask Price"],
		)
	case 'k', 'K': // Quote Replace
		fmt.Fprintf(w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", msgType,
			kvInt["Bid Reference Number Delta"],
			kvInt["Original Bid Reference Number Delta"],
			kvInt["Bid Size"],
			kvInt["Bid Price"],
		)
		fmt.Fprintf(w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", msgType,
			kvInt["Ask Reference Number Delta"],
			kvInt["Original Ask Reference Number Delta"],
			kvInt["Ask Size"],
			kvInt["Ask Price"],
		)
	case 'Y': // Quote Delete
		fmt.Fprintf(w, "%s %c %08x %08x %08x %08x\n",
			"NORM QBID", msgType,
			kvInt["Bid Reference Number Delta"],
		)
		fmt.Fprintf(w, "%s %c %08x %08x %08x %08x\n",
			"NORM QASK", msgType,
			kvInt["Ask Reference Number Delta"],
		)
	case 'a', 'A': // Add Order
		fmt.Fprintf(w, "%s %c %c %08x %08x %08x %08x\n",
			"NORM ORDER", msgType,
			kvInt["Market Side"],
			kvInt["Option ID"],
			kvInt["Order Reference Number Delta"],
			kvInt["Volume"],
			kvInt["Price"],
		)
	case 'E': // Single Side Executed
		fmt.Fprintf(w, "%s %c %08x %08x\n",
			"NORM ORDER", msgType,
			kvInt["Reference Number Delta"],
			kvInt["Volume"],
		)
	case 'C': // Single Side Executed with Price
		fmt.Fprintf(w, "%s %c %08x %08x\n",
			"NORM ORDER", msgType,
			kvInt["Reference Number Delta"],
			kvInt["Executed Contracts"],
		)
	case 'X': //  Order Cancel
		fmt.Fprintf(w, "%s %c %08x %08x\n",
			"NORM ORDER", msgType,
			kvInt["Order Reference Number Delta"],
			kvInt["Cancelled Contracts"],
		)
	case 'G': // Single Side Update
		fmt.Fprintf(w, "%s %c %08x %08x %08x\n",
			"NORM ORDER", msgType,
			kvInt["Reference Number Delta"],
			kvInt["Volume"],
			kvInt["Price"],
		)
	case 'u', 'U': // Single Side Replace
		fmt.Fprintf(w, "%s %c %08x %08x %08x %08x\n",
			"NORM ORDER", msgType,
			kvInt["New Reference Number Delta"],
			kvInt["Original Reference Number Delta"],
			kvInt["Volume"],
			kvInt["Price"],
		)
	case 'D': // Single Side Delete
		fmt.Fprintf(w, "%s %c %08x\n",
			"NORM ORDER", msgType,
			kvInt["Reference Number Delta"],
		)
	default:
		log.Fatalf("Unknown message type %d (%c)\n", msgType, msgType)
	}
}

func translate(r io.Reader, w io.Writer) {
	kvRegexp := regexp.MustCompile("(?m)^            ([^:]*): (.*)$")
	parValueRegexp := regexp.MustCompile(".*\\((\\d+)\\)")
	scanner := bufio.NewScanner(r)
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
			kvStr := make(map[string]string)
			kvInt := make(map[string]uint)
			for _, m := range matches {
				k := m[1]
				v := m[2]
				if _, ok := kvStr[k]; ok {
					pretty.Println(ittoMessage)
					pretty.Println(matches)
					pretty.Println(m)
					log.Fatal("Duplicate key ", k)
				}
				kvStr[k] = v
				vInt, err := strconv.ParseUint(v, 0, 32)
				if err == nil {
					kvInt[k] = uint(vInt)
				} else if matches := parValueRegexp.FindStringSubmatch(v); matches != nil {
					vInt, err := strconv.ParseUint(matches[1], 0, 32)
					kvInt[k] = uint(vInt)
					if err != nil {
						log.Fatal("Can't parse", v)
					}
				}
			}
			//pretty.Println(kvStr)
			//pretty.Println(kvInt)
			//outMessage1(w, kvStr, kvInt)
			outMessage2(w, kvStr, kvInt)
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
	translate(dumpReader, outFile)
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
	translate(os.Stdin, os.Stdout)
	_ = pretty.Print
	_ = fmt.Print

}
