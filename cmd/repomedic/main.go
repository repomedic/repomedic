package main

import (
	"repomedic/internal/cli"
	_ "repomedic/internal/fetcher/providers"
	_ "repomedic/internal/rules/checks"
)

// These variables are populated by the build via -ldflags (see Taskfile.yml).
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cli.SetBuildInfo(version, commit, date)
	cli.Execute()
}
