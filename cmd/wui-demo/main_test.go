// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRunWritesPNGToFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "shot.png")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	// PNG magic: 89 50 4E 47 0D 0A 1A 0A
	if len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("output is not a PNG: %q", data[:min(8, len(data))])
	}
}

func TestRunDarkTheme(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "shot.png")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--out", out, "--theme", "dark"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, stderr.String())
	}
}

func TestRunStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--out", "-"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatalf("expected PNG bytes on stdout")
	}
}

func TestRunFlagParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--bogus"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run exit=%d, want 1", code)
	}
}

func TestRunOutOpenError(t *testing.T) {
	// nonexistent directory → os.Create fails
	var stdout, stderr bytes.Buffer
	code := run([]string{"--out", "/nonexistent_dir_xyz/shot.png"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run exit=%d, want 1", code)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ io.Writer = (*bytes.Buffer)(nil)
