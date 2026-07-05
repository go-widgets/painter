// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRunEmitsANSI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, stderr.String())
	}
	s := stdout.String()
	if !strings.Contains(s, "\x1b[38;2;") {
		t.Fatalf("expected 24-bit ANSI fg sequence in output")
	}
	if !strings.Contains(s, "\x1b[48;2;") {
		t.Fatalf("expected 24-bit ANSI bg sequence in output")
	}
}

func TestRunDarkTheme(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--theme", "dark"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, stderr.String())
	}
}

func TestRunFlagParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--bogus"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run exit=%d, want 1", code)
	}
}

// errWriter fails every write — used to exercise the WriteANSI
// error branch of run().
type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) { return 0, errors.New("boom") }

func TestRunReportsWriteError(t *testing.T) {
	var stderr bytes.Buffer
	if code := run(nil, errWriter{}, &stderr); code != 1 {
		t.Fatalf("run exit=%d, want 1", code)
	}
}

// TestMainSuccessPath drives main() through the runFunc/osExit seams so
// os.Exit is not actually invoked and the main() function itself gets
// covered.
func TestMainSuccessPath(t *testing.T) {
	origRun, origExit := runFunc, osExit
	defer func() { runFunc, osExit = origRun, origExit }()
	got := -1
	runFunc = func([]string, io.Writer, io.Writer) int { return 0 }
	osExit = func(code int) { got = code }
	main()
	if got != 0 {
		t.Fatalf("main() called osExit(%d), want 0", got)
	}
}

func TestMainErrorPath(t *testing.T) {
	origRun, origExit := runFunc, osExit
	defer func() { runFunc, osExit = origRun, origExit }()
	got := -1
	runFunc = func([]string, io.Writer, io.Writer) int { return 1 }
	osExit = func(code int) { got = code }
	main()
	if got != 1 {
		t.Fatalf("main() called osExit(%d), want 1", got)
	}
}
