package main

import (
	"os"

	"github.com/opencloud-eu/opencloud-music/pkg/command"
	"github.com/opencloud-eu/opencloud-music/pkg/config/defaults"
)

func main() {
	cfg := defaults.FullDefaultConfig()
	if err := command.Execute(cfg); err != nil {
		os.Exit(1)
	}
}
