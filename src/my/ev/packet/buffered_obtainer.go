// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"io"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"my/errs"
)

type BufferedObtainer struct {
	index    int
	data     [][]byte
	ci       []gopacket.CaptureInfo
	linkType layers.LinkType
}

func NewBufferedObtainer(p Obtainer) *BufferedObtainer {
	b := &BufferedObtainer{}
	for {
		data, ci, err := p.ReadPacketData()
		if err == io.EOF {
			break
		}
		errs.CheckE(err)
		b.data = append(b.data, data)
		b.ci = append(b.ci, ci)
	}
	b.linkType = p.LinkType()
	return b
}
func (b *BufferedObtainer) ReadPacketData() (data []byte, ci gopacket.CaptureInfo, err error) {
	if b.index >= len(b.data) {
		err = io.EOF
		return
	}
	data = b.data[b.index]
	ci = b.ci[b.index]
	b.index++
	return
}
func (b *BufferedObtainer) ZeroCopyReadPacketData() (data []byte, ci gopacket.CaptureInfo, err error) {
	return b.ReadPacketData()
}
func (b *BufferedObtainer) Reset() {
	//runtime.GC()
	b.index = 0
}
func (b *BufferedObtainer) LinkType() layers.LinkType {
	return b.linkType
}
func (b *BufferedObtainer) Packets() int {
	return len(b.ci)
}
