// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"encoding/binary"
	"io"
	"log"
	"os"

	"github.com/ikravets/errs"
	"github.com/jessevdk/go-flags"

	"my/ev/rec"
)

type cmdSoupbin2memh struct {
	InputFileName string `long:"input" short:"i" required:"y" value-name:"FILE" description:"soupbintcp data stream"`
	DestDirName   string `short:"d" long:"dest-dir" default:"." default-mask:"current dir" value-name:"DIR" description:"destination directory, will be created if does not exist" `
	Limit         int    `long:"count" short:"c" value-name:"NUM" description:"limit number of input records"`
	shouldExecute bool
}

func (c *cmdSoupbin2memh) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdSoupbin2memh) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("soupbin2memh", "convert soupbin file to readmemh simulator input", "", c)
}

func (c *cmdSoupbin2memh) ParsingFinished() (err error) {
	if !c.shouldExecute {
		return
	}
	inputFile, err := os.OpenFile(c.InputFileName, os.O_RDONLY, 0644)
	errs.CheckE(err)
	defer inputFile.Close()

	printer, err := rec.NewMemhRecorder(c.DestDirName)
	errs.CheckE(err)
	defer func() { errs.CheckE(printer.Close()) }()
	printer.AddDummy()

	var buf []byte
	for i := 0; i < c.Limit || c.Limit == 0; i++ {
		var header struct {
			Size uint16
			Type byte
		}
		err := binary.Read(inputFile, binary.BigEndian, &header)
		if err == io.EOF {
			break
		}
		errs.CheckE(err)
		payloadSize := header.Size - 1
		sendSize := int(payloadSize) + 2
		if sendSize > cap(buf) {
			buf = make([]byte, sendSize)
		}
		buf = buf[:sendSize]
		n, err := inputFile.Read(buf[2:])
		errs.CheckE(err)
		errs.Check(n == int(payloadSize), n, payloadSize)
		binary.BigEndian.PutUint16(buf, payloadSize)
		if header.Type == 'S' {
			errs.CheckE(printer.AddData(buf))
		} else {
			log.Printf("record type '%c' != 'S'\n", header.Type)
		}
	}
	return
}

func init() {
	var c cmdSoupbin2memh
	Registry.Register(&c)
}
