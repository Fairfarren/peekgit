package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultIntervalSec = 300
	DefaultConcurrency = 3
)

type WorkspaceMap map[string][]string

type GlobalConfig struct {
	Workspaces WorkspaceMap `json:"workspaces"`
}

type Config struct {
	IntervalSec    int
	Concurrency    int
	NoGitHub       bool
	ShowVersion    bool
	WorkspaceMode  bool
	WorkspaceDepth int
	WorkspaceRoot  string
	Global         GlobalConfig
}

// LoadGlobalConfig reads ~/.config/peekgit/config.json
func LoadGlobalConfig() (GlobalConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return GlobalConfig{}, err
	}
	configPath := filepath.Join(home, ".config", "peekgit", "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return GlobalConfig{Workspaces: make(WorkspaceMap)}, nil
		}
		return GlobalConfig{}, err
	}

	var cfg GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return GlobalConfig{}, err
	}

	expandTilde(&cfg, home)
	return cfg, nil
}

func expandTilde(cfg *GlobalConfig, home string) {
	for wsName, paths := range cfg.Workspaces {
		for i, p := range paths {
			switch {
			case p == "~":
				cfg.Workspaces[wsName][i] = home
			case strings.HasPrefix(p, "~/"), strings.HasPrefix(p, "~\\"):
				cfg.Workspaces[wsName][i] = filepath.Join(home, p[2:])
			}
		}
	}
}

func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("peekgit", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "PeekGit - 终端里的多仓库监控面板\n\n")
		fmt.Fprintf(os.Stderr, "用法:\n  peekgit [参数]\n\n")
		fmt.Fprintf(os.Stderr, "参数:\n")
		fs.PrintDefaults()
	}

	interval := fs.Int("interval", DefaultIntervalSec, "自动刷新间隔（秒）")
	concurrency := fs.Int("concurrency", DefaultConcurrency, "并发 fetch 数量")
	noGitHub := fs.Bool("no-github", false, "禁用 GitHub 功能（PR、Issues）")
	workspaceDepth := fs.Int("workspaces", 0, "扫描当前目录为工作区；支持可选深度（默认深度为 1）")
	showVersion := fs.Bool("version", false, "显示版本信息并退出")
	vShort := fs.Bool("v", false, "显示版本信息并退出")

	if err := fs.Parse(normalizeWorkspaceArgs(args)); err != nil {
		return Config{}, err
	}

	validateLimits(interval, concurrency)

	workspaceMode := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "workspaces" {
			workspaceMode = true
		}
	})

	if *showVersion || *vShort {
		return Config{ShowVersion: true}, nil
	}

	if workspaceMode {
		return buildWorkspaceConfig(*interval, *concurrency, *noGitHub, *workspaceDepth)
	}

	globalCfg, err := LoadGlobalConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		IntervalSec:    *interval,
		Concurrency:    *concurrency,
		NoGitHub:       *noGitHub,
		WorkspaceMode:  false,
		WorkspaceDepth: 0,
		WorkspaceRoot:  "",
		Global:         globalCfg,
	}, nil
}

func validateLimits(interval, concurrency *int) {
	if *interval <= 0 {
		*interval = DefaultIntervalSec
	}
	if *concurrency <= 0 {
		*concurrency = DefaultConcurrency
	}
}

var getwd = os.Getwd

func buildWorkspaceConfig(interval, concurrency int, noGitHub bool, workspaceDepth int) (Config, error) {
	depth := workspaceDepth
	if depth <= 0 {
		depth = 0
	}
	wd, err := getwd()
	if err != nil {
		return Config{}, err
	}
	root := filepath.Clean(wd)
	global := GlobalConfig{
		Workspaces: WorkspaceMap{
			root: {root},
		},
	}
	return Config{
		IntervalSec:    interval,
		Concurrency:    concurrency,
		NoGitHub:       noGitHub,
		WorkspaceMode:  true,
		WorkspaceDepth: depth,
		WorkspaceRoot:  root,
		Global:         global,
	}, nil
}

func normalizeWorkspaceArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-workspaces" || arg == "--workspaces" {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				out = append(out, arg+"=1")
				continue
			}
		}
		out = append(out, arg)
	}
	return out
}
