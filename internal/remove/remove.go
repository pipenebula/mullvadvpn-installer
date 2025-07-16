package remove

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/you/mullvad-installer/internal/config"
	initpkg "github.com/you/mullvad-installer/internal/init"
	"github.com/you/mullvad-installer/internal/ui"
)

func Remove(cfg *config.Config) error {
	initSys := initpkg.Detect()
	svcName := "mullvad-daemon"

	switch initSys {
	case initpkg.Systemd:
		ui.Info("Detected systemd → stopping and disabling Mullvad service")
		if !cfg.DryRun {
			_ = runCmd("systemctl", "stop", svcName+".service")
			_ = runCmd("systemctl", "disable", svcName+".service")
			_ = os.Remove("/etc/systemd/system/" + svcName + ".service")
			_ = runCmd("systemctl", "daemon-reload")
		}

	case initpkg.Runit:
		ui.Info("Detected runit → stopping Mullvad service")
		if !cfg.DryRun {
			_ = runCmd("sv", "stop", svcName)
			_ = os.Remove("/var/service/" + svcName)
			_ = os.RemoveAll("/etc/sv/" + svcName)
		}

	case initpkg.SysV:
		ui.Info("Detected SysV init → stopping and removing init.d script")
		if !cfg.DryRun {
			_ = runCmd("/etc/init.d/"+svcName, "stop")
			_ = os.Remove("/etc/init.d/" + svcName)
		}

	case initpkg.OpenRC:
		ui.Info("Detected OpenRC → stopping and removing service")
		if !cfg.DryRun {
			_ = runCmd("rc-service", svcName, "stop")
			_ = runCmd("rc-update", "del", svcName, "default")
			_ = os.Remove("/etc/init.d/" + svcName)
		}

	case initpkg.S6:
		ui.Info("Detected s6 → removing service directory")
		if !cfg.DryRun {
			_ = os.RemoveAll("/etc/s6/" + svcName)
			_ = os.Remove("/var/service/" + svcName)
		}

	case initpkg.Dinit:
		ui.Info("Detected dinit → stopping and removing service")
		if !cfg.DryRun {
			_ = runCmd("dinitctl", "stop", svcName)
			_ = os.Remove("/etc/dinit.d/" + svcName)
			_ = runCmd("dinitctl", "reload")
		}

	default:
		ui.Info("Unknown init system → skipping service stop/removal")
	}

	additionalPaths := []string{
		"/usr/share/bash-completion/completions/mullvad",
		"/usr/share/icons/hicolor/32x32/apps/mullvad-vpn.png",
		"/usr/share/icons/hicolor/48x48/apps/mullvad-vpn.png",
		"/usr/share/icons/hicolor/64x64/apps/mullvad-vpn.png",
		"/usr/share/icons/hicolor/128x128/apps/mullvad-vpn.png",
		"/usr/share/icons/hicolor/256x256/apps/mullvad-vpn.png",
		"/usr/share/icons/hicolor/512x512/apps/mullvad-vpn.png",
		"/usr/share/icons/hicolor/1024x1024/apps/mullvad-vpn.png",
		"/usr/share/fish/vendor_completions.d/mullvad.fish",
		"/usr/share/doc/mullvad-vpn",
		"/usr/share/applications/mullvad-vpn.desktop",
		"/usr/local/share/zsh/site-functions/_mullvad",
		"/usr/bin/mullvad",
		"/usr/bin/mullvad-daemon",
		"/usr/bin/mullvad-exclude",
		"/usr/bin/mullvad-problem-report",
		"/opt/Mullvad VPN",
	}

	for _, path := range additionalPaths {
		ui.Info("Removing ", path)
		if cfg.DryRun {
			ui.Info("  (dry-run) skipping")
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %q: %w", path, err)
		}
	}

	ui.Info("Uninstallation complete.")
	return nil
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
