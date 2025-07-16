package init

import (
	"os"
	"path/filepath"
	"strings"
)

type InitSystem string

const (
	Systemd InitSystem = "systemd"
	Runit   InitSystem = "runit"
	SysV    InitSystem = "sysvinit"
	OpenRC  InitSystem = "openrc"
	S6      InitSystem = "s6"
	Dinit   InitSystem = "dinit"
	Unknown InitSystem = "unknown"
)

func Detect() InitSystem {
	procComm, _ := os.ReadFile("/proc/1/comm")
	exePath, _ := os.Readlink("/proc/1/exe")
	has := func(p string) bool { _, err := os.Stat(p); return err == nil }

	baseComm := strings.TrimSpace(string(procComm))
	exeName := filepath.Base(exePath)

	switch {
	case baseComm == "systemd":
		return Systemd
	case contains([]string{"runit", "runsvinit", "runsvdir"}, []string{baseComm, exeName}) ||
		(has("/etc/sv") && has("/etc/service")):
		return Runit
	case baseComm == "init" && has("/etc/init.d") && has("/etc/rc.d"):
		return SysV
	case baseComm == "openrc" || (has("/etc/init.d") && has("/run/openrc")):
		return OpenRC
	case baseComm == "s6-svscan" || has("/etc/s6"):
		return S6
	case baseComm == "dinit" || has("/etc/dinit"):
		return Dinit
	default:
		return Unknown
	}
}

func contains(vals, tests []string) bool {
	for _, t := range tests {
		for _, v := range vals {
			if t == v {
				return true
			}
		}
	}
	return false
}
