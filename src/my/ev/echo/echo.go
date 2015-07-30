// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package echo

import (
	"container/list"
	"io"
	"log"
	"net"

	"my/errs"
)

type EchoServer interface {
	Run()
}

func NewEchoServer() EchoServer {
	return &echoServer{
		laddr: ":7000",
	}
}

type echoServer struct {
	laddr string
}

func (s *echoServer) Run() {
	l, err := net.Listen("tcp", s.laddr)
	errs.CheckE(err)
	defer l.Close()
	for {
		conn, err := l.Accept()
		errs.CheckE(err)
		log.Printf("accepted %s -> %s \n", conn.RemoteAddr(), conn.LocalAddr())
		go s.handleClient(conn)
	}
}

type msg struct {
	data []byte
}

func (s *echoServer) handleClient(conn net.Conn) {
	defer conn.Close()
	rch := make(chan msg, 1000)
	sch := make(chan msg, 1000)
	done := make(chan error)
	l := list.New()
	rchOk := true
	var m msg

	go receiver(conn, rch)
	go sender(conn, sch, done)

	for rchOk || l.Len() > 0 {
		if l.Len() == 0 {
			if m, rchOk = <-rch; rchOk {
				l.PushBack(m)
			}
		} else if !rchOk {
			sch <- l.Front().Value.(msg)
			l.Remove(l.Front())
		} else {
			front := l.Front()
			select {
			case m, rchOk = <-rch:
				if rchOk {
					l.PushBack(m)
				}
			case sch <- front.Value.(msg):
				l.Remove(front)
			}
		}
	}
	close(sch)
	<-done
	log.Printf("closed %s -> %s \n", conn.RemoteAddr(), conn.LocalAddr())
}
func receiver(conn net.Conn, ch chan<- msg) {
	for {
		b := make([]byte, 1<<12)
		n, err := conn.Read(b)
		if err != nil {
			if err != io.EOF {
				log.Printf("Warning: error during receive: %s\n", err)
			}
			break
		}
		ch <- msg{data: b[:n]}
	}
	close(ch)
}
func sender(conn net.Conn, ch <-chan msg, done chan<- error) {
	var err error
	for msg := range ch {
		if err == nil {
			_, err = conn.Write(msg.data)
		}
	}
	if err != nil {
		log.Printf("Warning: error during send: %s\n", err)
	}
	done <- err
	close(done)
}
