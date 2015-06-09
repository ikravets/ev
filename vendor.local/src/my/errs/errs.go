// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package errs

import (
	"fmt"
	"runtime"
	"strconv"
)

type CheckerError interface {
	error
	OrigError() error
	Args() []interface{}
	Location() (file string, line int)
	StackTrace() []byte
	Checker() Checker
}

type checkerError struct {
	err        error
	args       []interface{}
	stackTrace []byte
	file       string
	line       int
	checker    Checker
}

func newCheckerError(callerDepth int, checker Checker, err error, args []interface{}) *checkerError {
	e := &checkerError{
		err:     err,
		args:    args,
		checker: checker,
	}
	_, e.file, e.line, _ = runtime.Caller(callerDepth + 1)
	return e
}
func (e *checkerError) Error() string {
	fileStr, lineStr, errStr := "<?>", "<?>", "<nil>"
	if e.file != "" {
		fileStr = e.file
	}
	if e.line != 0 {
		lineStr = strconv.Itoa(e.line)
	}
	if e.err != nil {
		errStr = e.err.Error()
	}
	return fmt.Sprintf("Check failed at %s:%s (err:%s, args=%#v)", fileStr, lineStr, errStr, e.args)
}
func (e *checkerError) OrigError() error {
	return e.err
}
func (e *checkerError) Args() []interface{} {
	return e.args
}
func (e *checkerError) Location() (string, int) {
	return e.file, e.line
}
func (e *checkerError) StackTrace() []byte {
	return e.stackTrace
}
func (e *checkerError) Checker() Checker {
	return e.checker
}

type Checker interface {
	CheckE(err error, v ...interface{})
	Check(cond bool, v ...interface{})
	PassE(errptr *error)
	Is(Checker) bool
	Catch(func(CheckerError))
}

type checkerLight struct {
	baseCallerDepth int
}

func NewCheckerLight(baseCallerDepth int) Checker {
	return &checkerLight{
		baseCallerDepth: baseCallerDepth,
	}
}

func (c *checkerLight) CheckE(err error, args ...interface{}) {
	if err != nil {
		panic(newCheckerError(c.baseCallerDepth+1, c, err, args))
	}
}
func (c *checkerLight) Check(cond bool, args ...interface{}) {
	if !cond {
		panic(newCheckerError(c.baseCallerDepth+1, c, nil, args))
	}
}
func (c *checkerLight) PassE(errptr *error) {
	r := recover()
	if r == nil {
		return
	}
	ce := r.(*checkerError)
	if ce == nil {
		// XXX no way to keep stack trace when re-panicing :(
		panic(r)
	}
	if errptr == nil {
		return
	}
	if ce.err != nil {
		*errptr = ce.err
	} else {
		*errptr = ce
	}
}
func (c *checkerLight) Catch(f func(CheckerError)) {
	r := recover()
	if r == nil {
		return
	}
	ce := r.(*checkerError)
	if ce == nil {
		// XXX no way to keep stack trace when re-panicing :(
		panic(r)
	}
	f(ce)
}
func (c *checkerLight) Is(checker Checker) bool {
	return checker.(*checkerLight) == c
}

type AssertError struct {
	checkerError
}

func (ae *AssertError) Error() string {
	if ae.file == "" {
		return fmt.Sprintf("assertion failed at <unknown>")
	} else {
		return fmt.Sprintf("assertion failed at %s:%d", ae.file, ae.line)
	}
}

func Assert(cond bool, args ...interface{}) {
	if cond {
		return
	}
	ae := AssertError{checkerError: *newCheckerError(1, nil, nil, args)}
	ae.stackTrace = make([]byte, 1<<20)
	runtime.Stack(ae.stackTrace, false)
	panic(&ae)
}

var defaultChecker = NewCheckerLight(1)

func CheckE(err error, args ...interface{}) {
	defaultChecker.CheckE(err, args...)
}
func Check(cond bool, args ...interface{}) {
	defaultChecker.Check(cond, args...)
}
func PassE(errptr *error) {
	defaultChecker.PassE(errptr)
}

/*
type CheckError struct {
	err error
	aux []interface{}
}

func Checker_() func(errptr *error) {
	nop := func(*error, ...interface{}) {}
	return CheckerX(nop)
}

func CheckerX(f func(errptr *error, v ...interface{})) func(errptr *error) {
	return func(errptr *error) {
		if r := recover(); r != nil {
			if ce, ok := r.(CheckError); ok {
				*errptr = ce.err
				f(errptr, ce.aux...)
			} else {
				// XXX no way to keep stack trace when re-panicing :(
				panic(r)
			}
		}
	}
}

func Check(err error, v ...interface{}) {
	if err != nil {
		panic(CheckError{err: err, aux: v})
	}
}
*/
