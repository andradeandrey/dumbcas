/* Copyright 2012 Marc-Antoine Ruel. Licensed under the Apache License, Version
2.0 (the "License"); you may not use this file except in compliance with the
License.  You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0. Unless required by applicable law or
agreed to in writing, software distributed under the License is distributed on
an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
or implied. See the License for the specific language governing permissions and
limitations under the License. */

package main

import (
	"bytes"
	//"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"testing"
)

// Logging is a global object so it can't be checked for when tests are run in parallel.
var bufLog bytes.Buffer

var enableOutput = false

func init() {
	// Reduces output. Comment out to get more logs.
	if !enableOutput {
		log.SetOutput(&bufLog)
	}
	log.SetFlags(log.Lmicroseconds)
}

type TB struct {
	*testing.T
	bufLog bytes.Buffer
	bufOut bytes.Buffer
	bufErr bytes.Buffer
	log    *log.Logger
}

func MakeTB(t *testing.T) *TB {
	tb := &TB{T: t}
	tb.log = log.New(&tb.bufLog, "", log.Lmicroseconds)
	if enableOutput {
		tb.Verbose()
	}
	return tb
}

func PrintIf(b []byte, name string) {
	s := strings.TrimSpace(string(b))
	if len(s) != 0 {
		fmt.Fprintf(os.Stderr, "\n\\/ \\/ %s \\/ \\/\n%s\n/\\ /\\ %s /\\ /\\\n", name, s, name)
	}
}

// Prints the stack trace to ease debugging.
// It's slightly slower than an explicit condition in the test but its more compact.
func (t *TB) Assertf(truth bool, fmt string, values ...interface{}) {
	if !truth {
		PrintIf(t.bufOut.Bytes(), "STDOUT")
		PrintIf(t.bufErr.Bytes(), "STDERR")
		PrintIf(t.bufLog.Bytes(), "LOG")
		PrintIf(debug.Stack(), "STACK")
		t.Fatalf(fmt, values...)
	}
}

func (t *TB) CheckBuffer(out, err bool) {
	if out {
		// Print Stderr to see what happened.
		t.Assertf(t.bufOut.Len() != 0, "Expected stdout")
	} else {
		t.Assertf(t.bufOut.Len() == 0, "Unexpected stdout")
	}

	if err {
		t.Assertf(t.bufErr.Len() != 0, "Expected stderr")
	} else {
		t.Assertf(t.bufErr.Len() == 0, "Unexpected stderr")
	}
	t.bufOut.Reset()
	t.bufErr.Reset()
}

func (tb *TB) Verbose() {
	if tb.bufLog.Len() != 0 {
		os.Stderr.Write(tb.bufLog.Bytes())
	}
	tb.log = log.New(os.Stderr, "", log.Lmicroseconds)
}

type ApplicationMock struct {
	DefaultApplication
	*TB
}

func (a *ApplicationMock) GetOut() io.Writer {
	return &a.bufOut
}

func (a *ApplicationMock) GetErr() io.Writer {
	return &a.bufErr
}

type CommandMock struct {
	Command
	//flags *flag.FlagSet
}

/*
func (c *CommandMock) GetFlags() *flag.FlagSet {
	return c.flags
}*/

func MakeAppMock(t *testing.T) *ApplicationMock {
	a := &ApplicationMock{application, MakeTB(t)}
	for i, c := range a.Commands {
		a.Commands[i] = &CommandMock{c}
	}
	return a
}

func TestHelp(t *testing.T) {
	t.Parallel()
	a := MakeAppMock(t)
	args := []string{"help"}
	r := Run(a, args)
	a.Assertf(r == 0, "Unexpected return code %d", r)
	a.CheckBuffer(true, false)
}

func TestHelpBadFlag(t *testing.T) {
	t.Parallel()
	a := MakeAppMock(t)
	args := []string{"help", "-foo"}
	// TODO(maruel): This is inconsistent.
	r := Run(a, args)
	a.Assertf(r == 0, "Unexpected return code %d", r)
	a.CheckBuffer(false, true)
}

func TestHelpBadCommand(t *testing.T) {
	t.Parallel()
	a := MakeAppMock(t)
	args := []string{"help", "non_existing_command"}
	r := Run(a, args)
	a.Assertf(r == 2, "Unexpected return code %d", r)
	a.CheckBuffer(false, true)
}

func TestBadCommand(t *testing.T) {
	t.Parallel()
	a := MakeAppMock(t)
	args := []string{"non_existing_command"}
	r := Run(a, args)
	a.Assertf(r == 2, "Unexpected return code %d", r)
	a.CheckBuffer(false, true)
}
