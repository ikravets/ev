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

	"github.com/ikravets/errs"

	"my/ev/packet"
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
	defer errs.PassE(&err)
	p = &MemhRecorder{dir: dir}
	p.hexDigits = []byte("0123456789abcdef")
	p.outbuf = bytes.NewBuffer(make([]byte, 0, 4096))
	errs.CheckE(os.MkdirAll(p.dir, 0755))
	indexFileName := filepath.Join(p.dir, "packet.length")
	p.indexFile, err = os.OpenFile(indexFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	errs.CheckE(err)
	return
}

func (p *MemhRecorder) Close() (err error) {
	defer errs.PassE(&err)
	_, err = fmt.Fprintf(p.indexFile, "%x\n", p.packetNum)
	errs.CheckE(err)
	for _, l := range p.packetLengths {
		_, err = fmt.Fprintf(p.indexFile, "%x\n", l)
		errs.CheckE(err)
	}
	p.indexFile.Close()
	return
}

func (p *MemhRecorder) AddData(data []byte) (err error) {
	defer errs.PassE(&err)
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

	p.packetLengths = append(p.packetLengths, len(data))
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
