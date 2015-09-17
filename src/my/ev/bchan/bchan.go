// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package bchan

import (
	"log"
	"sync/atomic"
)

type BchanConsumer interface {
	Chan() chan interface{}
	Close()
}

type Bchan interface {
	ProducerChan() chan interface{}
	NewConsumer() BchanConsumer
	Close()
}

type bchan struct {
	prod chan interface{}
	cons map[*bchanCons]struct{}
}

func NewBchan() Bchan {
	b := &bchan{
		prod: make(chan interface{}, 1),
		cons: make(map[*bchanCons]struct{}),
	}
	go b.run()
	return b
}

func (b *bchan) ProducerChan() chan interface{} {
	return b.prod
}
func (b *bchan) NewConsumer() BchanConsumer {
	ch := &bchanCons{
		ch: make(chan interface{}, 1),
	}
	b.cons[ch] = struct{}{}
	return ch
}
func (b *bchan) Close() {
	close(b.prod)
}
func (b *bchan) run() {
	for {
		val, ok := <-b.prod
		if !ok {
			log.Printf("producer channel closed")
			break
		}
		//log.Printf("produced %#v", val)
		for cons := range b.cons {
			//log.Printf("consider consumer %#v", cons)
			if atomic.LoadInt32(&cons.closed) != 0 {
				close(cons.ch)
				delete(b.cons, cons)
				continue
			}
			select {
			case cons.ch <- val:
				//log.Printf("value sent to consumer %#v", cons)
			default:
			}
		}
	}
	for cons := range b.cons {
		close(cons.ch)
		delete(b.cons, cons)
	}
}

type bchanCons struct {
	ch     chan interface{}
	closed int32
}

func (c *bchanCons) Chan() chan interface{} {
	return c.ch
}
func (c *bchanCons) Close() {
	atomic.StoreInt32(&c.closed, 1)
}
