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

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	globalCfg, err := LoadGlobalConfig()
	if err != nil {
		return Config{}, err
	}

	if *interval <= 0 {
		*interval = DefaultIntervalSec
	}
	if *concurrency <= 0 {
		*concurrency = DefaultConcurrency
	}

	return Config{
		IntervalSec: *interval,
		Concurrency: *concurrency,
		NoGitHub:    *noGitHub,
		Global:      globalCfg,
	}, nil
}
