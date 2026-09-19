package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var runProgram = func(model tea.Model, opts ...tea.ProgramOption) error {
	allOpts := append([]tea.ProgramOption{tea.WithAltScreen()}, opts...)
	p := tea.NewProgram(model, allOpts...)
	_, err := p.Run()
	return err
}

var osExit = os.Exit

func main() {
	code := run(os.Args[1:], os.Stderr)
	osExit(code)
}

func run(args []string, errOut io.Writer) int {
	cfg, err := config.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(errOut, "参数解析失败: %v\n", err)
		return 2
	}

	if cfg.ShowVersion {
		fmt.Printf("peekgit version %s (commit: %s, date: %s)\n", version, commit, date)
		return 0
	}

	app := tui.New(cfg)
	if err := runProgram(app); err != nil {
		fmt.Fprintf(errOut, "运行失败: %v\n", err)
		return 1
	}
	return 0
}
