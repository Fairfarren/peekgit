package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
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

func TestRunHelpFlag(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	code := run([]string{"-h"}, buf)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if buf.Len() != 0 {
		t.Fatalf("unexpected error output: %q", buf.String())
	}
}

func TestRunSuccess(t *testing.T) {
	orig := runProgram
	runProgram = func(_ tea.Model, _ ...tea.ProgramOption) error { return nil }
	defer func() { runProgram = orig }()

	buf := bytes.NewBuffer(nil)
	code := run([]string{"--no-github"}, buf)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
}

func TestRunProgramFailure(t *testing.T) {
	orig := runProgram
	runProgram = func(_ tea.Model, _ ...tea.ProgramOption) error { return errors.New("boom") }
	defer func() { runProgram = orig }()

	buf := bytes.NewBuffer(nil)
	code := run([]string{"--no-github"}, buf)
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
}

func TestRunVersion(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	code := run([]string{"--version"}, buf)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
}

func TestMainFunction(t *testing.T) {
	origExit := osExit
	origProgram := runProgram
	origArgs := os.Args
	defer func() {
		osExit = origExit
		runProgram = origProgram
		os.Args = origArgs
	}()

	os.Args = []string{"peekgit", "--no-github"}
	runProgram = func(_ tea.Model, _ ...tea.ProgramOption) error { return nil }

	exitCode := -1
	osExit = func(code int) {
		exitCode = code
	}

	main()
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}

type immediateQuitModel struct{}

func (immediateQuitModel) Init() tea.Cmd                       { return tea.Quit }
func (immediateQuitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return immediateQuitModel{}, tea.Quit }
func (immediateQuitModel) View() string                        { return "" }

func TestDefaultRunProgram(t *testing.T) {
	err := runProgram(immediateQuitModel{}, tea.WithoutRenderer(), tea.WithInput(strings.NewReader("")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
