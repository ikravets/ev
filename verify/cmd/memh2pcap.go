// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/layers"
	"code.google.com/p/gopacket/pcapgo"
	"github.com/jessevdk/go-flags"

	"my/errs"

	"my/itto/verify/packet/moldudp64"
)

func init() {
	var c cmdMemh2pcap
	Registry.Register(&c)
}

type cmdMemh2pcap struct {
	OutputFileName string `long:"output" short:"o" required:"y" value-name:"PCAP_FILE" description:"output pcap file"`
	shouldExecute  bool
	inputFileNames []string
}

func (c *cmdMemh2pcap) Execute(args []string) error {
	c.shouldExecute = true
	c.inputFileNames = args
	return nil
}

func (c *cmdMemh2pcap) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("memh2pcap", "convert readmemh file to pcap", "", c)
}

func (c *cmdMemh2pcap) ParsingFinished() {
	if !c.shouldExecute {
		return
	}
	out, err := os.OpenFile(c.OutputFileName, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	errs.CheckE(err)
	defer out.Close()
	m2p := new(memh2pcap)
	m2p.Open(out)

	for _, fn := range c.inputFileNames {
		fi, err := os.Stat(fn)
		errs.CheckE(err)
		if fi.IsDir() {
			fis, err := ioutil.ReadDir(fn)
			errs.CheckE(err)
			sort.Sort(SortedFiles(fis))
			for _, fi = range fis {
				errs.CheckE(m2p.addFile(filepath.Join(fn, fi.Name())))
			}
		} else {
			errs.CheckE(m2p.addFile(fn))
		}
	}
}

type SortedFiles []os.FileInfo

func (a SortedFiles) Len() int      { return len(a) }
func (a SortedFiles) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a SortedFiles) Less(i, j int) bool {
	extract := func(name string) (int, error) {
		trimmed := strings.TrimRightFunc(name, unicode.IsDigit)
		suf := name[len(trimmed):]
		return strconv.Atoi(suf)
	}
	numi, erri := extract(a[i].Name())
	numj, errj := extract(a[j].Name())
	switch {
	case erri == nil && errj == nil:
		return numi < numj
	case erri != nil && errj != nil:
		return a[i].Name() < a[j].Name()
	default:
		return erri == nil
	}
}

type memh2pcap struct {
	pw     *pcapgo.Writer
	ci     gopacket.CaptureInfo
	so     gopacket.SerializeOptions
	sb     gopacket.SerializeBuffer
	num    int
	mold   *moldudp64.MoldUDP64
	data   bytes.Buffer
	layers []gopacket.SerializableLayer
}

func (m *memh2pcap) Open(w io.Writer) (err error) {
	errs.PassE(&err)
	m.pw = pcapgo.NewWriter(w)
	errs.CheckE(m.pw.WriteFileHeader(65536, layers.LinkTypeEthernet))

	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr([]byte{0, 22, 33, 44, 55, 66}),
		DstMAC:       net.HardwareAddr([]byte{11, 22, 33, 44, 55, 77}),
		EthernetType: layers.EthernetType(0x800),
	}
	ip := &layers.IPv4{
		Version:  4,
		SrcIP:    net.IP{1, 2, 3, 4},
		DstIP:    net.IP{233, 54, 12, 1},
		TTL:      1,
		Protocol: layers.IPProtocolUDP,
	}
	udp := &layers.UDP{
		SrcPort: 1,
		DstPort: 18001,
	}
	errs.CheckE(udp.SetNetworkLayerForChecksum(ip))
	mold := &moldudp64.MoldUDP64{
		Session:        "TestSess00",
		SequenceNumber: 0,
		MessageCount:   1,
	}
	moldMb := &moldudp64.MoldUDP64MessageBlockChained{}

	m.mold = mold
	m.layers = append([]gopacket.SerializableLayer{}, moldMb, mold, udp, ip, eth)

	m.sb = gopacket.NewSerializeBuffer()
	m.so = gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	return
}
func (m *memh2pcap) addOne(r io.Reader) (err error) {
	errs.PassE(&err)
	m.data.Reset()
	scanner := bufio.NewScanner(r)
	for i := 0; scanner.Scan(); i++ {
		v, err := strconv.ParseUint(scanner.Text(), 16, 64)
		errs.CheckE(err)
		line := make([]byte, 8)
		binary.LittleEndian.PutUint64(line, v)
		if i == 0 {
			// skip pseudo header (2 bytes)
			line = line[2:]
		}
		_, err = m.data.Write(line)
		errs.CheckE(err)
	}

	m.sb.Clear()
	b, err := m.sb.AppendBytes(m.data.Len())
	errs.CheckE(err)
	copy(b, m.data.Bytes())
	for _, l := range m.layers {
		errs.CheckE(l.SerializeTo(m.sb, m.so))
	}
	ci := gopacket.CaptureInfo{
		Timestamp:     time.Unix(int64(m.mold.SequenceNumber), 1),
		CaptureLength: len(m.sb.Bytes()),
		Length:        len(m.sb.Bytes()),
	}
	errs.CheckE(m.pw.WritePacket(ci, m.sb.Bytes()))
	m.mold.SequenceNumber++
	return
}
func (m *memh2pcap) addFile(filename string) (err error) {
	log.Printf("adding %s\n", filename)
	errs.PassE(&err)
	in, err := os.Open(filename)
	errs.CheckE(err)
	errs.CheckE(m.addOne(in))
	errs.CheckE(in.Close())
	return
}
