package main

import (
	"bytes"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRunInvalidFlag(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	code := run([]string{"--unknown"}, buf)
	if code != 2 {
		t.Fatalf("code=%d", code)
	}
}

func TestRunSuccess(t *testing.T) {
	orig := runProgram
	runProgram = func(_ tea.Model) error { return nil }
	defer func() { runProgram = orig }()

	buf := bytes.NewBuffer(nil)
	code := run([]string{"--no-github"}, buf)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
}

func TestRunProgramFailure(t *testing.T) {
	orig := runProgram
	runProgram = func(_ tea.Model) error { return errors.New("boom") }
	defer func() { runProgram = orig }()

	buf := bytes.NewBuffer(nil)
	code := run([]string{"--no-github"}, buf)
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
}
