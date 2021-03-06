// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package efh

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/ikravets/errs"

	"my/ev/channels"
	"my/ev/inspect"
	"my/ev/packet"
)

type ReplayConfig struct {
	InputFileName   string
	OutputInterface string
	Pps             int
	Limit           int
	Loop            int

	EfhLoglevel  int
	EfhIgnoreGap bool
	EfhDump      string
	EfhSubscribe []string
	EfhChannel   channels.Config
	EfhProf      bool

	RegConfig *inspect.Config

	TestEfh string
	Local   bool
}

type EfhReplay interface {
	Run() (err error)
}

type efhReplay struct {
	ReplayConfig
	testEfhCmd    *exec.Cmd
	testEfhDump   string
	testEfhArgs   []string
	testEfhExit   error
	testEfhDoneCh chan struct{}
	replay        packet.Replay
	replayDoneCh  chan struct{}
}

func NewEfhReplay(conf ReplayConfig) EfhReplay {
	return &efhReplay{
		ReplayConfig: conf,
	}
}

var (
	DumpsDifferError = errors.New("dumps differ")
	BadProbeError    = errors.New("register probe is bad")
)

func (e *efhReplay) Run() (err error) {
	defer errs.PassE(&err)
	if e.EfhDump != "" {
		e.testEfhDump = "test_efh.dump"
	}
	defaultTestEfh := "/usr/libexec/test_efh"
	if e.Local {
		defaultTestEfh = os.ExpandEnv("$HOME/efh-install/bin/test_efh")
		e.testEfhArgs = append(e.testEfhArgs, "--load-fw", "/usr/share/efh/firmware/fw.bin")
	}
	if e.TestEfh == "" {
		e.TestEfh = defaultTestEfh
	}
	errs.CheckE(e.startTestEfh())
	errs.CheckE(e.startDumpReplay())
	func() {
		ticker := time.NewTicker(time.Second / 10)
		defer ticker.Stop()
		for {
			select {
			case <-e.testEfhDoneCh:
				fmt.Println()
				return
			case <-e.replayDoneCh:
				fmt.Printf("\rdone: 100%%   \n")
				time.Sleep(100 * time.Millisecond)
				return
			case <-ticker.C:
				if done, ok := e.replay.Progress(); ok {
					fmt.Printf("\rdone: %.1f%%", done*100)
				}
			}
		}
	}()
	errs.CheckE(e.stopDumpReplay())
	errs.CheckE(e.stopTestEfh())
	if e.testEfhDump != "" {
		var same bool
		same, err = e.diffAppDump()
		errs.CheckE(err)
		if !same {
			log.Printf("dumps differ")
			err = DumpsDifferError
		}
	}
	if e.RegConfig != nil {
		errs.CheckE(e.RegConfig.Probe())
		errs.CheckE(ioutil.WriteFile("registers", []byte(e.RegConfig.Report()), 0666))
		if e.RegConfig.IsBad() {
			log.Printf("register probe is bad")
			if err == nil {
				err = BadProbeError
			}
		}
	}
	return
}

func (e *efhReplay) startTestEfh() (err error) {
	defer errs.PassE(&err)
	e.testEfhArgs = append(e.testEfhArgs,
		"--loglevel", strconv.Itoa(e.EfhLoglevel),
		"--logfile", "test_efh.log",
		"--net-mcast", "10.101.107.97",
	)
	if e.EfhProf {
		e.testEfhArgs = append(e.testEfhArgs, "--prof")
	}
	for _, ch := range e.EfhChannel.Addrs() {
		e.testEfhArgs = append(e.testEfhArgs, "--channel", ch)
	}
	if len(e.EfhSubscribe) == 0 {
		e.testEfhArgs = append(e.testEfhArgs, "--debug-assume-subscribed")
	} else {
		for _, s := range e.EfhSubscribe {
			e.testEfhArgs = append(e.testEfhArgs, "--subscribe", s)
		}
	}
	e.testEfhArgs = append(e.testEfhArgs,
		"--debug-stop-on-status", "STATUS_RX_QUEUE_FULL",
		"--debug-stop-on-status", "STATUS_DESTINATION_IP_UNKNOWN",
		"--debug-stop-on-status", "STATUS_BAD_FRAME",
	)
	if e.EfhIgnoreGap {
		e.testEfhArgs = append(e.testEfhArgs, "--debug-disable-gap")
	}
	if e.testEfhDump != "" {
		e.testEfhArgs = append(e.testEfhArgs, "--dump-file", e.testEfhDump)
	}
	readyFile := "ready"
	e.testEfhArgs = append(e.testEfhArgs, "--readycmd", fmt.Sprintf("{ sleep 5; touch %s;}&", readyFile))

	log.Printf("starting %s %s", e.TestEfh, e.testEfhArgs)
	e.testEfhCmd = exec.Command(e.TestEfh, e.testEfhArgs...)
	out, err := os.Create("test_efh.out")
	errs.CheckE(err)
	e.testEfhCmd.Stdout = out
	e.testEfhCmd.Stderr = out

	errs.CheckE(e.testEfhCmd.Start())
	e.testEfhDoneCh = make(chan struct{})
	go func() {
		e.testEfhExit = e.testEfhCmd.Wait()
		close(e.testEfhDoneCh)
	}()

	func() {
		for {
			select {
			case <-time.After(time.Second / 100):
				if _, err := os.Stat(readyFile); err == nil {
					os.Remove(readyFile)
					log.Printf("test_efh ready")
					return
				}
			case <-time.After(30 * time.Second):
				err = errors.New("timeout starting test_efh")
				return
			case <-e.testEfhDoneCh:
				err = errors.New("test_efh exited too soon")
				return
			}
		}
	}()
	return
}

func (e *efhReplay) stopTestEfh() (err error) {
	log.Printf("stopping test_efh\n")
	e.testEfhCmd.Process.Signal(os.Interrupt)
	time.AfterFunc(10*time.Second, func() { e.testEfhCmd.Process.Kill() })
	<-e.testEfhDoneCh
	if e.testEfhExit != nil {
		log.Printf("test_efh wait: %s\n", e.testEfhExit)
	}
	return
}

func (e *efhReplay) startDumpReplay() (err error) {
	conf := packet.ReplayConfig{
		IfaceName: e.OutputInterface,
		DumpName:  e.InputFileName,
		Limit:     e.Limit,
		Pps:       e.Pps,
		Loop:      e.Loop,
	}
	e.replay = packet.NewReplay(&conf)
	log.Printf("starting replay %v", e.replay)
	e.replayDoneCh = make(chan struct{})
	go func() {
		errs.CheckE(e.replay.Run())
		close(e.replayDoneCh)
	}()
	return
}

func (e *efhReplay) stopDumpReplay() (err error) {
	log.Printf("stopping replay\n")
	e.replay.Stop()
	<-e.replayDoneCh
	return
}

func (e *efhReplay) diffAppDump() (same bool, err error) {
	if e.testEfhDump == "" {
		return
	}
	log.Printf("compare output dumps [ %s, %s ]\n", e.EfhDump, e.testEfhDump)
	defer errs.PassE(&err)

	expFile, err := os.Open(e.EfhDump)
	errs.CheckE(err)
	defer expFile.Close()
	actFile, err := os.Open(e.testEfhDump)
	errs.CheckE(err)
	defer actFile.Close()

	expFi, err := expFile.Stat()
	errs.CheckE(err)
	actFi, err := actFile.Stat()
	errs.CheckE(err)
	if expFi.Size() != actFi.Size() {
		log.Printf("dump size differ: exp %d act %d", expFi.Size(), actFi.Size())
		return
	}

	expBytes := make([]byte, 1<<20)
	actBytes := make([]byte, 1<<20)
	pos := 0
	for {
		n1, err1 := io.ReadFull(expFile, expBytes)
		n2, err2 := io.ReadFull(actFile, actBytes)
		if n1 != n2 {
			log.Printf("dump read differ at pos %d", pos)
			return
		}
		if !bytes.Equal(expBytes[:n1], actBytes[:n1]) {
			log.Printf("dump bytes differ at pos >= %d", pos)
			return
		}
		if err1 == io.ErrUnexpectedEOF && err2 == io.ErrUnexpectedEOF || err1 == io.EOF && err2 == io.EOF {
			break
		}
		errs.CheckE(err1)
		errs.CheckE(err2)
		pos += n1
	}
	same = true
	log.Printf("dumps are the same")
	return
}
