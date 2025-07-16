package arch

import (
	"runtime"
)

type OSInfo struct {
	Arch string
}

func Detect() OSInfo {
	arch := runtime.GOARCH
	return OSInfo{Arch: mapArch(arch)}
}

func mapArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	default:
		return goarch
	}
}
