package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/you/mullvad-installer/internal/arch"
	"github.com/you/mullvad-installer/internal/config"
	"github.com/you/mullvad-installer/internal/github"
	initpkg "github.com/you/mullvad-installer/internal/init"
	"github.com/you/mullvad-installer/internal/installer"
	"github.com/you/mullvad-installer/internal/remove"
	"github.com/you/mullvad-installer/internal/ui"
	"github.com/you/mullvad-installer/internal/wizard"
)

const (
	fetchTimeout   = 10 * time.Second
	fetchRetries   = 3
	fetchBackoff   = 500 * time.Millisecond
	spinnerDots    = 3
	spinnerRefresh = 200 * time.Millisecond
)

func main() {
	if err := run(); err != nil {
		ui.Fatal(err)
	}
}

func run() error {
	cfg := config.ParseFlags()
	ui.InitLogger(cfg.NoColor)

	if os.Geteuid() != 0 {
		ui.Info("--help")
		return errors.New("Need to be root")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	u := ui.NewUI(os.Stdin, os.Stdout, os.Stderr, cfg.AssumeYes, cfg.DryRun, cfg.NoColor)

	userCtx, err := wizard.NewConfirmationWizard(cfg).Run(u)
	if err != nil {
		return fmt.Errorf("confirmation: %w", err)
	}
	if !userCtx.Confirmed {
		ui.Info("Aborted by user")
		return nil
	}

	if userCtx.DoRemove {
		ui.Info("Removing previous installation…")
		if err := remove.Remove(cfg); err != nil {
			return fmt.Errorf("remove: %w", err)
		}
		ui.Info("Old installation removed")
	}

	osInfo := arch.Detect()
	initSys := initpkg.Detect()

	rel, err := fetchRelease(ctx, u, userCtx.Channel)
	if err != nil {
		return fmt.Errorf("fetch release: %w", err)
	}
	ui.Info("Selected release:", rel.Tag)

	ui.Info("Installing…")
	if err := installer.Install(rel, osInfo, cfg, u, userCtx.UseSystemXZ); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := installer.SetupService(initSys, cfg); err != nil {
		ui.Warn("service setup warning:", err)
	}
	ui.Info("Installation complete")

	return nil
}

func fetchRelease(ctx context.Context, u *ui.UI, channel string) (*github.Release, error) {
	if err := u.RunAll(ui.Spinner("Fetching releases", spinnerDots, spinnerRefresh)); err != nil {
		return nil, err
	}

	ctxFetch, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	var lastErr error
	for i := 0; i < fetchRetries && ctxFetch.Err() == nil; i++ {
		rel, err := github.GetLatestRelease(channel)
		if err == nil {
			return rel, nil
		}
		lastErr = err
		time.Sleep(fetchBackoff)
	}
	if ctxFetch.Err() != nil {
		return nil, errors.New("fetch timeout exceeded")
	}
	return nil, fmt.Errorf("all retries failed: %w", lastErr)
}
