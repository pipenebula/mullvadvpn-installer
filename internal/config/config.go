package config

import (
	"flag"
	ui "github.com/you/mullvad-installer/internal/ui"
	"os"
)

type ActionType string

const (
	ActionInstall ActionType = "install"
	ActionRemove  ActionType = "remove"
	ActionUpgrade ActionType = "upgrade"
)

type Config struct {
	AssumeYes bool
	DryRun    bool
	NoColor   bool
	ForceAll  bool
	Action    ActionType
	Channel   string // stable|beta
}

var (
	flagYes      bool
	flagDryRun   bool
	flagNoColor  bool
	flagForceAll bool
	flagChannel  string
)

func init() {
	flag.BoolVar(&flagYes, "yes", false, "assume yes to all prompts")
	flag.BoolVar(&flagDryRun, "dry-run", false, "show actions but do not execute")
	flag.BoolVar(&flagNoColor, "no-color", false, "disable colored output")
	flag.BoolVar(&flagForceAll, "force-remove-all", false, "skip all remove prompts (implies --yes)")
	flag.StringVar(&flagChannel, "channel", "", "release channel: stable|beta (if omitted, will prompt)")
}

func ParseFlags() *Config {
	flag.Parse()

	act := ActionInstall
	for _, a := range os.Args[1:] {
		switch a {
		case "--remove":
			act = ActionRemove
		case "--upgrade":
			act = ActionUpgrade
		}
	}

	cfg := &Config{
		AssumeYes: flagYes || flagForceAll,
		DryRun:    flagDryRun,
		NoColor:   flagNoColor,
		ForceAll:  flagForceAll,
		Action:    act,
		Channel:   flagChannel,
	}

	if cfg.Action != ActionRemove && cfg.Channel == "" {
		cfg.Channel = ui.MsgSelectChannel
	}
	if cfg.Channel == "" {
		cfg.Channel = "stable"
	}
	return cfg
}
