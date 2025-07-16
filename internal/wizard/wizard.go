package wizard

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/you/mullvad-installer/internal/config"
	"github.com/you/mullvad-installer/internal/ui"
)

type UserContext struct {
	*config.Config
	InstalledVersion string
	Channel          string
	Confirmed        bool
	DoRemove         bool
	UseSystemXZ      bool
}

type Wizard struct {
	cfg *config.Config
}

func NewConfirmationWizard(cfg *config.Config) *Wizard {
	return &Wizard{cfg: cfg}
}

func detectStep(ctx *UserContext) ui.Step {
	return func(u *ui.UI) error {
		out, err := exec.Command("mullvad-daemon", "--version").Output()
		if err != nil {
			return nil
		}
		fields := strings.Fields(string(bytes.TrimSpace(out)))
		if len(fields) == 0 {
			return errors.New("cannot parse mullvad-daemon version output")
		}
		ver := fields[len(fields)-1]
		ctx.InstalledVersion = ver
		ui.Info("Detected installed Mullvad VPN version:", ver)
		return ui.ConfirmfLazy(
			"Mullvad VPN %s is already installed. Upgrade instead?",
			ctx.AssumeYes,
			func(ok bool) {
				if !ok {
					ui.Info("Aborted by user")
					os.Exit(0)
				}
				ctx.DoRemove = true
				ctx.ForceAll = true
			},
			func() any { return ver },
		)(u)
	}
}

func (w *Wizard) Run(u *ui.UI) (*UserContext, error) {
	ctx := &UserContext{Config: w.cfg}

	pre := []ui.Step{
		ui.LogStep(ui.Hi),
		detectStep(ctx),
	}
	if err := u.RunAll(pre...); err != nil {
		return nil, err
	}

	initSteps := []ui.Step{
		ui.SelectStableBeta(ui.MsgSelectChannel, func(ch string) {
			ctx.Channel = ch
		}),
		ui.ConfirmfLazy(
			ui.MsgConfirmAction,
			w.cfg.AssumeYes,
			func(ok bool) { ctx.Confirmed = ok },
			func() any { return w.cfg.Action },
			func() any { return ctx.Channel },
		),
	}
	if err := u.RunAll(initSteps...); err != nil {
		return nil, err
	}
	if !ctx.Confirmed {
		ui.Info("Aborted by user")
		os.Exit(0)
	}

	followup := []ui.Step{
		ui.Conditional{
			Cond: func() bool { return !ctx.DoRemove },
			S: ui.Confirm(
				ui.MsgRemoveOld,
				w.cfg.ForceAll,
				func(ok bool) {
					if !ok {
						ui.Info("Aborted by user")
						os.Exit(0)
					}
					ctx.DoRemove = ok
					w.cfg.ForceAll = ok
				},
			),
		}.Run,
		ui.SelectXZBackend(ui.MsgSelectXZBackend, func(sys bool) {
			ctx.UseSystemXZ = sys
		}),
	}
	if err := u.RunAll(followup...); err != nil {
		return nil, err
	}

	return ctx, nil
}
