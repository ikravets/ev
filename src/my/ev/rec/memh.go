// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package rec

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"my/errs"
	"my/itto/verify/packet"
)

type MemhRecorder struct {
	dir           string
	packetNum     int
	indexFile     io.WriteCloser
	packetLengths []int
	hexDigits     []byte
	outbuf        *bytes.Buffer
}

func NewMemhRecorder(dir string) (p *MemhRecorder, err error) {
	errs.PassE(&err)
	p = &MemhRecorder{dir: dir}
	p.hexDigits = []byte("0123456789abcdef")
	p.outbuf = bytes.NewBuffer(make([]byte, 0, 4096))
	errs.CheckE(os.MkdirAll(p.dir, 0755))
	indexFileName := filepath.Join(p.dir, "packet.length")
	p.indexFile, err = os.OpenFile(indexFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	errs.CheckE(err)
	return
}

func (p *MemhRecorder) Close() {
	fmt.Fprintf(p.indexFile, "%x\n", p.packetNum)
	for _, l := range p.packetLengths {
		fmt.Fprintf(p.indexFile, "%x\n", l)
	}
	p.indexFile.Close()
}

func (p *MemhRecorder) AddData(data []byte) (err error) {
	errs.PassE(&err)
	p.outbuf.Reset()
	chunk := make([]byte, 8)
	zeroChunk := make([]byte, 8)
	for i := 0; i < len(data); i += len(chunk) {
		if len(data)-i < len(chunk) {
			copy(chunk, zeroChunk)
		}
		copy(chunk, data[i:])
		for i := len(chunk) - 1; i >= 0; i-- {
			b := chunk[i]
			errs.CheckE(p.outbuf.WriteByte(p.hexDigits[b/16]))
			errs.CheckE(p.outbuf.WriteByte(p.hexDigits[b%16]))
		}
		errs.CheckE(p.outbuf.WriteByte('\n'))
	}

	packetLength := (len(data) + len(chunk) - 1) / len(chunk)
	p.packetLengths = append(p.packetLengths, packetLength)
	dataFileName := filepath.Join(p.dir, fmt.Sprintf("packet.readmemh%d", p.packetNum))
	errs.CheckE(ioutil.WriteFile(dataFileName, p.outbuf.Bytes(), 0644))
	p.packetNum++
	return
}

func (p *MemhRecorder) AddDummy() error {
	zeroData := make([]byte, 64)
	return p.AddData(zeroData)
}

var _ packet.Handler = &MemhRecorder{}

func (p *MemhRecorder) HandlePacket(packet packet.Packet) {
	errs.CheckE(p.AddData(packet.Data()))
}
func (_ *MemhRecorder) HandleMessage(_ packet.ApplicationMessage) {}
