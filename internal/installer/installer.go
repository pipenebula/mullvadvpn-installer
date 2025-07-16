package installer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/you/mullvad-installer/internal/arch"
	"github.com/you/mullvad-installer/internal/config"
	"github.com/you/mullvad-installer/internal/debpkg"
	"github.com/you/mullvad-installer/internal/github"
	initpkg "github.com/you/mullvad-installer/internal/init"
	"github.com/you/mullvad-installer/internal/service"
	"github.com/you/mullvad-installer/internal/ui"
)

func Install(
	rel *github.Release,
	osInfo arch.OSInfo,
	cfg *config.Config,
	u *ui.UI,
	useSystemXZ bool,
) error {
	assetURL, err := selectDebAsset(rel, osInfo.Arch)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "mullvad-")
	if err != nil {
		return fmt.Errorf("make temp dir: %w", err)
	}
	RegisterTmpDir(tmpDir)
	defer func() {
		_ = os.RemoveAll(tmpDir)
		UnregisterTmpDir(tmpDir)
	}()

	debPath := filepath.Join(tmpDir, "package.deb")
	ui.Info("Downloading URL:", assetURL)
	if cfg.DryRun {
		ui.Info("(dry-run) would download", assetURL, "→", debPath)
	} else {
		if err := fetchFile(u, assetURL, debPath, cfg); err != nil {
			return err
		}
	}

	assetName := filepath.Base(assetURL)
	version := strings.TrimPrefix(rel.Tag, "MullvadVPN-")
	base := strings.TrimSuffix(assetName, ".deb")
	sigURL := fmt.Sprintf(
		"https://cdn.mullvad.net/app/desktop/releases/%s/%s.deb.asc",
		version, base,
	)
	ui.Info("Verifying PGP signature of ", assetName, " via CDN…")

	if cfg.DryRun {
		ui.Info("Skipping PGP signature verification (dry-run)")
	} else {
		if err := verifyPGP(debPath, sigURL); err != nil {
			return fmt.Errorf("pgp signature verification failed for %s: %w", assetName, err)
		}
		ui.Info("PGP signature OK")
	}

	extractDir := filepath.Join(tmpDir, "ex")
	if cfg.DryRun {
		ui.Info("(dry-run) would extract .deb from", debPath, "to", extractDir)
	} else {
		if err := debpkg.ExtractDeb(debPath, extractDir, useSystemXZ); err != nil {
			return fmt.Errorf("extract .deb: %w", err)
		}
		ui.Info("Extracted .deb to", extractDir)
	}

	for _, pair := range []struct{ src, dst string }{
		{filepath.Join(extractDir, "opt"), "/opt"},
		{filepath.Join(extractDir, "usr"), "/usr"},
	} {
		if cfg.DryRun {
			ui.Info("(dry-run) would copy tree from", pair.src, "to", pair.dst)
		} else {
			ui.Info("Installing tree from", pair.src, "→", pair.dst)
			if err := installTree(pair.src, pair.dst, cfg); err != nil {
				return err
			}
		}
	}

	return nil
}

func selectDebAsset(rel *github.Release, arch string) (string, error) {
	for _, a := range rel.Assets {
		if strings.Contains(a.Name, arch+".deb") {
			return a.URL, nil
		}
	}
	return "", fmt.Errorf("no .deb for arch %q", arch)
}

func fetchFile(
	u *ui.UI,
	url, dest string,
	cfg *config.Config,
) error {
	if cfg.DryRun {
		ui.Info(fmt.Sprintf("(dry-run) would download  %s → %s", url, dest))
		return nil
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	pr := newProgressReader(resp.Body, resp.ContentLength)
	if _, err := io.Copy(out, pr); err != nil {
		return fmt.Errorf("copy download: %w", err)
	}
	pr.finishPrint()

	return nil
}

func installTree(src, dst string, cfg *config.Config) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			if cfg.DryRun {
				ui.Info(fmt.Sprintf("(dry-run) mkdir %s", target))
				return nil
			}
			return os.MkdirAll(target, info.Mode())
		}

		ui.Info("Installing file", target)
		if cfg.DryRun {
			return nil
		}
		return copyFileWithMode(path, target, info.Mode())
	})
}

func copyFileWithMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("open dst %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s→%s: %w", src, dst, err)
	}
	return nil
}

type progressReader struct {
	reader    io.Reader
	total     int64
	read      int64
	start     time.Time
	lastPrint time.Time
}

func newProgressReader(r io.Reader, total int64) *progressReader {
	now := time.Now()
	return &progressReader{
		reader:    r,
		total:     total,
		start:     now,
		lastPrint: now,
	}
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)
	if n > 0 {
		p.read += int64(n)
	}
	if time.Since(p.lastPrint) > 200*time.Millisecond || err == io.EOF {
		elapsed := time.Since(p.start)
		ui.Progress(p.read, p.total, elapsed)
		p.lastPrint = time.Now()
	}
	return n, err
}

func (p *progressReader) finishPrint() {
	elapsed := time.Since(p.start)
	ui.FinishProgress(p.read, p.total, elapsed)
}

func SetupService(initSys initpkg.InitSystem, cfg *config.Config) error {
	if cfg.DryRun {
		switch initSys {
		case initpkg.Systemd:
			ui.Info("(dry-run) would install systemd unit")
		case initpkg.Runit:
			ui.Info("(dry-run) would install runit service")
		case initpkg.SysV:
			ui.Info("(dry-run) would install sysvinit script")
		case initpkg.OpenRC:
			ui.Info("(dry-run) would install openrc service")
		case initpkg.S6:
			ui.Info("(dry-run) would install s6 service")
		case initpkg.Dinit:
			ui.Info("(dry-run) would install dinit unit")
		default:
			ui.Info("(dry-run) unsupported init system, would skip")
		}
		return nil
	}

	switch initSys {
	case initpkg.Systemd,
		initpkg.Runit,
		initpkg.SysV,
		initpkg.OpenRC,
		initpkg.S6,
		initpkg.Dinit:
		if err := service.SetupService(cfg); err != nil {
			return fmt.Errorf("service setup for %s failed: %w", initSys, err)
		}
		ui.Info("Service for %s installed and started", initSys)
		return nil
	default:
		ui.Info("Unsupported init system %q, skipping service setup", initSys)
		return nil
	}
}
