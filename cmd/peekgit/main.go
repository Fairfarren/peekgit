package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

var runProgram = func(model tea.Model) error {
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func main() {
	code := run(os.Args[1:], os.Stderr)
	os.Exit(code)
}

func run(args []string, errOut io.Writer) int {
	cfg, err := config.Parse(args)
	if err != nil {
		if _, writeErr := fmt.Fprintf(errOut, "参数解析失败: %v\n", err); writeErr != nil {
			return 2
		}
		return 2
	}

	app := tui.New(cfg)
	if err := runProgram(app); err != nil {
		if _, writeErr := fmt.Fprintf(errOut, "运行失败: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}
	return 0
}
