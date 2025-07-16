package service

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/you/mullvad-installer/internal/config"
	initpkg "github.com/you/mullvad-installer/internal/init"
)

//go:embed templates/*
var templates embed.FS

func SetupService(cfg *config.Config) error {
	sys := initpkg.Detect()

	switch sys {
	case initpkg.Systemd:
		return writeFileAndRun(
			"/etc/systemd/system/mullvad-daemon.service",
			"templates/systemd/unit",
			[]string{"systemctl daemon-reload"},
			[]string{"systemctl enable --now mullvad-daemon.service"},
		)

	case initpkg.Runit:
		return setupDirService("runit", "/etc/sv/mullvad-daemon", func(svcDir string) error {
			link := "/var/service/mullvad-daemon"
			_ = os.RemoveAll(link)
			return os.Symlink(svcDir, link)
		}, []string{"sv up /var/service/mullvad-daemon"})

	case initpkg.SysV:
		return writeFileAndRun(
			"/etc/init.d/mullvad-daemon",
			"templates/sysvinit/init.d",
			nil,
			[]string{"/etc/init.d/mullvad-daemon start"},
		)

	case initpkg.OpenRC:
		return writeFileAndRun(
			"/etc/init.d/mullvad-daemon",
			"templates/openrc/service",
			nil,
			[]string{"rc-update add mullvad-daemon default", "rc-service mullvad-daemon start"},
		)

	case initpkg.S6:
		return setupDirService("s6", "/etc/s6/mullvad-daemon", nil, nil)

	case initpkg.Dinit:
		return writeFileAndRun(
			"/etc/dinit.d/mullvad.daemon",
			"templates/dinit/mullvad.daemon",
			[]string{"dinitctl reload"},
			nil,
		)

	default:
		return fmt.Errorf("unsupported init system: %s", sys)
	}
}

func writeFileAndRun(dest, tplPath string, pre, post []string) error {
	data, err := templates.ReadFile(tplPath)
	if err != nil {
		return fmt.Errorf("read template %q: %w", tplPath, err)
	}
	if err := os.WriteFile(dest, data, 0755); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	for _, cmd := range pre {
		if err := runShell(cmd); err != nil {
			return fmt.Errorf("pre cmd %q: %w", cmd, err)
		}
	}
	for _, cmd := range post {
		if err := runShell(cmd); err != nil {
			return fmt.Errorf("post cmd %q: %w", cmd, err)
		}
	}
	return nil
}

func setupDirService(name, svcDir string, hook func(string) error, post []string) error {
	base := fmt.Sprintf("templates/%s", name)
	if err := fs.WalkDir(templates, base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(base, path)
		dst := filepath.Join(svcDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		data, _ := templates.ReadFile(path)
		return os.WriteFile(dst, data, 0755)
	}); err != nil {
		return err
	}

	if hook != nil {
		if err := hook(svcDir); err != nil {
			return err
		}
	}
	for _, cmd := range post {
		if err := runShell(cmd); err != nil {
			return fmt.Errorf("post cmd %q: %w", cmd, err)
		}
	}
	return nil
}

func runShell(cmdLine string) error {
	parts := strings.Fields(cmdLine)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
