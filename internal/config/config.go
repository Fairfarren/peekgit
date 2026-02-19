package config

import (
	"flag"
	"os"
	"path/filepath"
)

const (
	DefaultIntervalSec = 300
	DefaultConcurrency = 3
)

type Config struct {
	Workspace   string
	IntervalSec int
	Concurrency int
	NoGitHub    bool
}

func Parse(args []string) (Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}

	fs := flag.NewFlagSet("repo-monitor", flag.ContinueOnError)
	workspace := fs.String("workspace", wd, "workspace path")
	interval := fs.Int("interval", DefaultIntervalSec, "refresh interval in seconds")
	concurrency := fs.Int("concurrency", DefaultConcurrency, "fetch concurrency")
	noGitHub := fs.Bool("no-github", false, "disable GitHub features")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	absWorkspace, err := filepath.Abs(*workspace)
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
		Workspace:   absWorkspace,
		IntervalSec: *interval,
		Concurrency: *concurrency,
		NoGitHub:    *noGitHub,
	}, nil
}
