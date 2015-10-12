// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ikravets/errs"
	"github.com/jessevdk/go-flags"

	"my/ev/efh"
)

type cmdEfhSuite struct {
	Exchange    string   `long:"exch"  short:"e" value-name:"EXCHANGE" default:"nasdaq" description:"nasdaq, bats, etc."`
	Suites      []string `long:"suite" short:"s" value-name:"SUITE"`
	Tests       []string `long:"test"  short:"t" value-name:"TEST"`
	Speed       int      `long:"speed" value-name:"NUM" default:"50000"`
	Limit       int      `long:"limit" value-name:"NUM"`
	Local       bool     `long:"local"`
	EfhLoglevel int      `long:"efh-loglevel" default:"6"`
	EfhProf     bool     `long:"efh-prof"`

	shouldExecute bool
	topOutDirName string
	testRunsTotal int
	testRunsOk    int
}

func (c *cmdEfhSuite) Execute(args []string) error {
	c.shouldExecute = true
	return nil
}

func (c *cmdEfhSuite) ConfigParser(parser *flags.Parser) {
	parser.AddCommand("efh_suite", "run test_efh test suite", "", c)
}

func (c *cmdEfhSuite) ParsingFinished() {
	defer errs.Catch(func(ce errs.CheckerError) {
		log.Printf("caught %s\n", ce)
	})
	if !c.shouldExecute {
		return
	}
	var err error
	c.topOutDirName = time.Now().Format("efh_regression.2006-01-02-15:04:05")
	suitesDirName := fmt.Sprintf("/local/dumps/%s/regression", c.Exchange)

	if len(c.Suites) == 0 || c.Suites[0] == "?" {
		suites, err := listDirs(suitesDirName)
		errs.CheckE(err)
		if len(c.Suites) > 0 {
			fmt.Printf("suites: %s\n", suites)
			return
		}
		if len(suites) == 0 {
			log.Println("Warning: no suites found")
			return
		}
		c.Suites = suites
	}

	for _, suite := range c.Suites {
		var suiteDirName string
		if strings.HasPrefix(suite, "/") {
			suiteDirName = suite
		} else if strings.HasPrefix(suite, "./") {
			cwd, err := os.Getwd()
			errs.CheckE(err)
			suiteDirName = filepath.Join(cwd, suite)
		} else {
			suiteDirName = filepath.Join(suitesDirName, suite)
		}
		var tests []string
		if len(c.Tests) == 0 || c.Tests[0] == "?" {
			tests, err = listDirs(suiteDirName)
			errs.CheckE(err)
			if len(c.Tests) > 0 {
				fmt.Printf("suite %s tests: %v\n", suiteDirName, tests)
				continue
			}
		} else {
			tests = c.Tests
		}
		if len(tests) == 0 {
			log.Printf("Warning: no tests found in suite %s\n", suite)
			continue
		}

		for _, testName := range tests {
			testDirName := filepath.Join(suiteDirName, testName)
			subscriptionFileNames, err := filepath.Glob(filepath.Join(testDirName, "subscription*"))
			errs.CheckE(err)
			if len(subscriptionFileNames) == 0 {
				c.RunTest(testDirName, nil)
			} else {
				for _, sfn := range subscriptionFileNames {
					a := strings.SplitAfter(sfn, "/subscription")
					suf := ""
					if len(a) > 0 {
						suf = a[len(a)-1]
					}
					c.RunTest(testDirName, &suf)
				}
			}
		}
	}
	log.Printf("Tests OK/Total: %d/%d\n", c.testRunsOk, c.testRunsTotal)
	fmt.Printf("Tests OK/Total: %d/%d\n", c.testRunsOk, c.testRunsTotal)
}
func (c *cmdEfhSuite) RunTest(testDirName string, suffix *string) (err error) {
	defer errs.Catch(func(ce errs.CheckerError) {
		log.Printf("caught %s\n", ce)
		err = ce
	})
	c.testRunsTotal++
	testRunName := time.Now().Format("2006-01-02-15:04:05.")
	tdnDir, tdnFile := filepath.Split(testDirName)
	tdnDir = filepath.Base(tdnDir)
	testRunName += tdnDir + "-" + tdnFile
	if suffix != nil {
		testRunName += *suffix
	}
	outDirName := filepath.Join(c.topOutDirName, testRunName)
	errs.CheckE(os.MkdirAll(outDirName, 0777))
	errs.CheckE(ioutil.WriteFile(filepath.Join(outDirName, "fail"), nil, 0666))
	expoutName := filepath.Join(testDirName, "expout-efh-orders")
	if suffix != nil {
		expoutName += *suffix
	}
	inputPcapName := filepath.Join(testDirName, "dump.pcap")
	_, err = os.Stat(expoutName)
	errs.CheckE(err)
	_, err = os.Stat(inputPcapName)
	errs.CheckE(err)
	efhDumpName := filepath.Join(outDirName, "expout_orders")
	errs.CheckE(os.Symlink(expoutName, efhDumpName))
	errs.CheckE(os.Symlink(testDirName, filepath.Join(outDirName, "dump_dir")))
	errs.CheckE(os.Symlink(inputPcapName, filepath.Join(outDirName, "dump.pcap")))
	conf := efh.ReplayConfig{
		InputFileName:   inputPcapName,
		OutputInterface: "eth1",
		Pps:             c.Speed,
		Limit:           c.Limit,
		Loop:            1,
		EfhLoglevel:     c.EfhLoglevel,
		EfhIgnoreGap:    true,
		EfhDump:         "expout_orders",
		EfhChannel:      c.genEfhChannels(testDirName),
		EfhProf:         c.EfhProf,
		TestEfh:         "/usr/libexec/test_efh",
		Local:           c.Local,
	}
	if suffix != nil {
		subscr := "subscription" + *suffix
		errs.CheckE(os.Symlink(filepath.Join(testDirName, subscr), filepath.Join(outDirName, subscr)))
		conf.EfhSubscribe = []string{subscr}
	}
	origWd, err := os.Getwd()
	errs.CheckE(err)
	errs.CheckE(os.Chdir(outDirName))
	er := efh.NewEfhReplay(conf)
	efhReplayErr := er.Run()
	errs.CheckE(os.Chdir(origWd))
	errs.CheckE(efhReplayErr)
	errs.CheckE(ioutil.WriteFile(filepath.Join(outDirName, "ok"), nil, 0666))
	errs.CheckE(os.Remove(filepath.Join(outDirName, "fail")))
	c.testRunsOk++
	return
}
func (c *cmdEfhSuite) genEfhChannels(testDirName string) (channels []string) {
	// TODO support testDirName/channels
	switch c.Exchange {
	case "nasdaq":
		for i := 0; i < 4; i++ {
			channels = append(channels, fmt.Sprintf("233.54.12.%d:%d", 1+i, 18001+i))
		}
	case "bats":
		for i := 0; i < 32; i++ {
			channels = append(channels, fmt.Sprintf("224.0.131.%d:%d", i/4, 30101+i))
		}
	default:
		errs.Check(false)
	}
	return
}

func listDirs(parent string) (children []string, err error) {
	defer errs.PassE(&err)
	matches, err := filepath.Glob(parent + "/*")
	errs.CheckE(err)
	for _, f := range matches {
		fi, err := os.Stat(f)
		errs.CheckE(err)
		if fi.IsDir() {
			children = append(children, fi.Name())
		}
	}
	return
}

func init() {
	var c cmdEfhSuite
	Registry.Register(&c)
}
