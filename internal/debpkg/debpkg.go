package debpkg

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ulikunitz/xz"
)

const (
	arMagicSize    = 8
	arHeaderSize   = 60
	arNameField    = 16
	arSizeOffset   = 48
	arSizeField    = 10
	defaultDirPerm = 0o755
)

var (
	ErrBadInput    = errors.New("invalid input path")
	ErrPathOutside = errors.New("path outside of destination")
	ErrBadLink     = errors.New("invalid symlink target")
	ErrXZNotFound  = errors.New("system xz not found")
	ErrArNotFound  = errors.New("system ar not found")
)

func ExtractDeb(debPath, dest string, useSystem bool) (retErr error) {
	if strings.TrimSpace(debPath) == "" || strings.TrimSpace(dest) == "" {
		return ErrBadInput
	}

	absDest, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("abs dest: %w", err)
	}

	var dataStream io.Reader
	if useSystem {
		dataStream, err = newSystemArStream(debPath, "data.tar.xz")
	} else {
		var f *os.File
		f, err = os.Open(debPath)
		if err != nil {
			return fmt.Errorf("open .deb: %w", err)
		}
		defer func() {
			if cerr := f.Close(); cerr != nil {
				if retErr == nil {
					retErr = fmt.Errorf("close .deb: %w", cerr)
				} else {
					fmt.Fprintf(os.Stderr, "warning: close .deb: %v\n", cerr)
				}
			}
		}()
		if _, err = f.Seek(arMagicSize, io.SeekStart); err != nil {
			return fmt.Errorf("seek magic: %w", err)
		}
		dataStream, err = findDataTarXz(f)
	}
	if err != nil {
		return err
	}

	var tarStream io.Reader
	if useSystem {
		tarStream, err = newSystemXZReader(dataStream)
	} else {
		tarStream, err = newGoXZReader(dataStream)
	}
	if err != nil {
		return fmt.Errorf("xz reader: %w", err)
	}

	tr := tar.NewReader(tarStream)
	return extractAll(tr, absDest)
}

func newSystemArStream(debPath, member string) (io.Reader, error) {
	cmd := exec.Command("ar", "p", debPath, member)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrArNotFound, err)
	}
	go func() {
		cmd.Wait()
		pw.Close()
	}()
	return pr, nil
}

func findDataTarXz(r io.ReadSeeker) (io.Reader, error) {
	buf := make([]byte, arHeaderSize)
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, fmt.Errorf("read ar header: %w", err)
		}
		name := strings.TrimRight(string(buf[:arNameField]), " /")
		sizeText := strings.TrimSpace(string(buf[arSizeOffset : arSizeOffset+arSizeField]))
		sz, err := strconv.ParseInt(sizeText, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse ar size: %w", err)
		}
		if name == "data.tar.xz" {
			return io.LimitReader(r, sz), nil
		}
		skip := sz
		if sz%2 != 0 {
			skip++
		}
		if _, err := r.Seek(skip, io.SeekCurrent); err != nil {
			return nil, fmt.Errorf("skip ar body: %w", err)
		}
	}
}

func newGoXZReader(r io.Reader) (io.Reader, error) {
	return xz.NewReader(r)
}

func newSystemXZReader(r io.Reader) (io.Reader, error) {
	cmd := exec.Command("xz", "-d", "-c")
	cmd.Stdin = r
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrXZNotFound, err)
	}
	go func() {
		cmd.Wait()
		pw.Close()
	}()
	return pr, nil
}

func extractAll(tr *tar.Reader, dest string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}
		if err := writeEntry(hdr, tr, dest); err != nil {
			return err
		}
	}
}

func writeEntry(hdr *tar.Header, tr *tar.Reader, dest string) (retErr error) {
	cleanName := filepath.Clean(hdr.Name)
	if cleanName == "." {
		return nil
	}
	if strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("%w: %s", ErrPathOutside, hdr.Name)
	}

	fullPath := filepath.Join(dest, cleanName)
	rel, err := filepath.Rel(dest, fullPath)
	if err != nil {
		return fmt.Errorf("invalid path %s: %w", fullPath, err)
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("%w: %s", ErrPathOutside, hdr.Name)
	}

	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(fullPath, hdr.FileInfo().Mode())

	case tar.TypeSymlink:
		target := hdr.Linkname
		if strings.Contains(target, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("%w: %s → %s", ErrBadLink, hdr.Name, hdr.Linkname)
		}
		if filepath.IsAbs(target) {
			target = strings.TrimPrefix(target, string(os.PathSeparator))
			target = filepath.Join(dest, target)
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), defaultDirPerm); err != nil {
			return fmt.Errorf("mkdir parent for symlink %s: %w", fullPath, err)
		}
		return os.Symlink(target, fullPath)

	case tar.TypeReg, tar.TypeLink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
		if err := os.MkdirAll(filepath.Dir(fullPath), defaultDirPerm); err != nil {
			return fmt.Errorf("mkdir parent for file %s: %w", fullPath, err)
		}
		out, err := os.OpenFile(fullPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fs.FileMode(hdr.Mode))
		if err != nil {
			return fmt.Errorf("open file %s: %w", fullPath, err)
		}
		defer func() {
			if cerr := out.Close(); cerr != nil {
				if retErr == nil {
					retErr = fmt.Errorf("close %s: %w", fullPath, cerr)
				} else {
					fmt.Fprintf(os.Stderr, "warning: close %s: %v\n", fullPath, cerr)
				}
			}
		}()
		if hdr.Typeflag == tar.TypeReg {
			if _, err := io.Copy(out, tr); err != nil {
				return fmt.Errorf("write file %s: %w", fullPath, err)
			}
		}
		return nil

	default:
		return nil
	}
}
