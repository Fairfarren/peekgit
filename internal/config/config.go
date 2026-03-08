package config

import (
	"encoding/json"
	"flag"
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
	IntervalSec int
	Concurrency int
	NoGitHub    bool
	WorkspaceMode  bool
	WorkspaceDepth int
	WorkspaceRoot  string
	Global      GlobalConfig
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

	// Expand ~ in workspace paths
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

	return cfg, nil
}

func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("peekgit", flag.ContinueOnError)
	interval := fs.Int("interval", DefaultIntervalSec, "refresh interval in seconds")
	concurrency := fs.Int("concurrency", DefaultConcurrency, "fetch concurrency")
	noGitHub := fs.Bool("no-github", false, "disable GitHub features")
	workspaceDepth := fs.Int("workspaces", 0, "scan current directory as workspace, optional depth")

	if err := fs.Parse(normalizeWorkspaceArgs(args)); err != nil {
		return Config{}, err
	}

	if *interval <= 0 {
		*interval = DefaultIntervalSec
	}
	if *concurrency <= 0 {
		*concurrency = DefaultConcurrency
	}

	workspaceMode := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "workspaces" {
			workspaceMode = true
		}
	})

	if workspaceMode {
		depth := *workspaceDepth
		if depth <= 0 {
			depth = 0
		}
		wd, err := os.Getwd()
		if err != nil {
			return Config{}, err
		}
		root, err := filepath.Abs(wd)
		if err != nil {
			return Config{}, err
		}
		global := GlobalConfig{
			Workspaces: WorkspaceMap{
				root: {root},
			},
		}
		return Config{
			IntervalSec:    *interval,
			Concurrency:    *concurrency,
			NoGitHub:       *noGitHub,
			WorkspaceMode:  true,
			WorkspaceDepth: depth,
			WorkspaceRoot:  root,
			Global:         global,
		}, nil
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
