// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package packet

import (
	"io"
	"log"
	"os/exec"
	"syscall"
)

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

func TsharkOpen(fileName string, args []string) (reader io.Reader, finisher func()) {
	var cmdArgs []string
	if len(fileName) > 0 {
		cmdArgs = append(cmdArgs, "-r", fileName)
	}
	cmdArgs = append(cmdArgs, args...)
	reader, finisher = startCommandPipe("tshark", cmdArgs, false)
	return
}
