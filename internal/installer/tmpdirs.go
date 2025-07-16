package installer

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	tmpDirsMu sync.Mutex
	tmpDirs   = make([]string, 0, 4)
)

func init() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		CleanupAll()
		os.Exit(1)
	}()
}

func RegisterTmpDir(dir string) {
	tmpDirsMu.Lock()
	tmpDirs = append(tmpDirs, dir)
	tmpDirsMu.Unlock()
}

func UnregisterTmpDir(dir string) {
	tmpDirsMu.Lock()
	defer tmpDirsMu.Unlock()
	for i, d := range tmpDirs {
		if d == dir {
			tmpDirs = append(tmpDirs[:i], tmpDirs[i+1:]...)
			return
		}
	}
}

func CleanupAll() {
	tmpDirsMu.Lock()
	defer tmpDirsMu.Unlock()
	for _, d := range tmpDirs {
		_ = os.RemoveAll(d)
	}
	tmpDirs = tmpDirs[:0]
}
