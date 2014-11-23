// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package pcap2memh

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/jessevdk/go-flags"
	"github.com/kr/pretty"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var textFrameSeparator []byte = []byte("\n\n0000 ")

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
		log.Println("WARNING skipping prefix:", string(data[:separatorIndex]))
		return separatorIndex, nil, nil
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

func translate(r io.Reader, dirPath string) {
	scanner := bufio.NewScanner(r)
	scanner.Split(splitTextFrames)
	hexdumpRegexp := regexp.MustCompile("(?m)^[[:xdigit:]]{4}  (([[:xdigit:]]{2} ){1,16})  ")
	outbuf := bytes.NewBuffer(make([]byte, 0, 4096))
	packetNum := 0
	packetLengths := make([]int, 0, 1024)
	for scanner.Scan() {
		//fmt.Println("=====================")
		//fmt.Println(scanner.Text())
		match := hexdumpRegexp.FindAllStringSubmatch(scanner.Text(), -1)
		packetLines := 0
		for i := range match {
			m := match[i][1]
			//fmt.Println(m)
			for j := 0; j < (len(m)/3+7)/8*8; j++ {
				var s string
				off := (j/8*8 + 7 - j%8) * 3
				if off < len(m)-2 {
					s = m[off : off+2]
				} else {
					s = "00"
				}
				outbuf.WriteString(s)
				if j%8 == 7 {
					outbuf.WriteByte('\n')
					packetLines++
				}
			}
		}
		//fmt.Println(outbuf.String())
		packetLengths = append(packetLengths, packetLines)
		dataFileName := filepath.Join(dirPath, fmt.Sprintf("packet.readmemh%d", packetNum))
		if err := ioutil.WriteFile(dataFileName, outbuf.Bytes(), 0644); err != nil {
			log.Fatal(err)
		}
		outbuf.Reset()
		packetNum++
	}

	outbuf.WriteString(fmt.Sprintf("%x\n", packetNum))
	for _, l := range packetLengths {
		outbuf.WriteString(fmt.Sprintf("%x\n", l))
	}
	indexFileName := filepath.Join(dirPath, "packet.length")
	if err := ioutil.WriteFile(indexFileName, outbuf.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
}

func getTsharkDump(fileName string, args []string) (reader io.Reader, finisher func()) {
	//pretty.Println(fileName, args)
	cmdArgs := []string{
		"-x",
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

type pcap2memh struct {
	DestDirName   string                        `short:"d" long:"dest-dir" default:"." default-mask:"current dir" value-name:"DIR" description:"destination directory, will be created if does not exist" `
	InputFileName string                        `long:"input" short:"i" required:"y" value-name:"PCAP_FILE" description:"input pcap file to read"`
	Args          struct{ TsharkArgs []string } `positional-args:"y"`
}

func (p *pcap2memh) Execute(args []string) error {
	//fmt.Println("pcap2memh Executed")
	//pretty.Println(p)
	//pretty.Println(args)
	dumpReader, finisher := getTsharkDump(p.InputFileName, p.Args.TsharkArgs)
	defer finisher()
	if err := os.MkdirAll(p.DestDirName, 0755); err != nil {
		log.Fatal(err)
	}
	translate(dumpReader, p.DestDirName)
	return nil
}

func InitArgv(parser *flags.Parser) {
	var p2m pcap2memh
	parser.AddCommand("pcap2memh",
		"convert pcap file to simulator input",
		"",
		&p2m)
}

/*****************************************************************************/
// experiments and debugging

func tryGoFlags() {
	var opts struct {
		Verbose []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
	}

	parser := flags.NewParser(&opts, flags.PassDoubleDash|flags.HelpFlag|flags.IgnoreUnknown)

	/*
		var p2m pcap2memh
		parser.AddCommand("pcap2memh",
			"convert pcap file to simulator input",
			"",
			&p2m)
	*/
	InitArgv(parser)
	_, err := parser.Parse()
	/*
		pretty.Println(err)
		pretty.Println(args)
		pretty.Println(p2m)
		pretty.Println(parser)
	*/
	if err != nil {
		perr := err.(*flags.Error)
		fmt.Println(perr.Message)
		return
	}
}

func tryCli() {
	app := cli.NewApp()
	app.Name = "et"
	app.Usage = "Ekaline tools"
	app.Commands = []cli.Command{
		{
			Name:            "pcap2mem",
			Usage:           "convert pcap file to simulator input",
			Description:     "specify any tshark arguments after \"--\" flag",
			SkipFlagParsing: true,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "input, pcap",
					Usage: "input pcap file to read (required)",
				},
				cli.StringFlag{
					Name:  "dest-dir",
					Value: ".",
					Usage: "destination directory (will be created)",
				},
			},
			Action: func(c *cli.Context) {
				pretty.Println(c.Args())
				destDir := c.String("dest-dir")
				inputFile := c.String("input")
				if inputFile == "" {
					log.Fatal("Must specify input file")
				}
				dumpReader, finisher := getTsharkDump(inputFile, c.Args())
				defer finisher()
				translate(dumpReader, destDir)
			},
		},
	}
	app.Run(os.Args)
}

func tryHardCoded() {
	tsharkArgs := []string{"-c", "1000"}
	dumpReader, finisher := getTsharkDump("/home/ilia/my/proj/ekaline/tmp/nasdaq/mcast.nasdaq.20140910.pcap", tsharkArgs)
	defer finisher()
	translate(dumpReader, "/tmp/packets")
}

func main() {
	tryGoFlags()
	_ = pretty.Print
	_ = fmt.Print
	_ = strconv.IntSize
	_ = strings.Contains
	_ = regexp.Compile

}
