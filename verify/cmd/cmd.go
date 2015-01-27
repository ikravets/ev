// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package cmd

import "github.com/jessevdk/go-flags"

type Extender interface {
	ConfigParser(*flags.Parser)
	ParsingFinished()
}

type ExtenderRegistry interface {
	Extender
	Register(Extender)
	Extenders() []Extender
}

type extenderRegistry struct {
	extenders []Extender
}

func (r *extenderRegistry) Register(e Extender) {
	r.extenders = append(r.extenders, e)
}

func (r *extenderRegistry) Extenders() []Extender {
	return r.extenders
}

func (r *extenderRegistry) ConfigParser(parser *flags.Parser) {
	for _, e := range r.extenders {
		e.ConfigParser(parser)
	}
}

func (r *extenderRegistry) ParsingFinished() {
	for _, e := range r.extenders {
		e.ParsingFinished()
	}
}

var Registry = extenderRegistry{}
