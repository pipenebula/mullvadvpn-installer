package ui

import (
	"fmt"
	"os"
	"time"
)

var noColorGlobal bool

func InitLogger(disableColor bool) {
	noColorGlobal = disableColor
}

func NoColor() bool {
	return noColorGlobal
}

func Hi(v ...any) {
	prefix := MsgWelcome
	if NoColor() {
		print(prefix, v...)
	} else {
		print(Text+prefix+Reset, v...)
	}
}

func Info(v ...any) {
	prefix := "INFO: "
	if NoColor() {
		print(prefix, v...)
	} else {
		print(Green+prefix+Reset, v...)
	}
}

func Warn(v ...any) {
	prefix := "WARN: "
	if NoColor() {
		print(prefix, v...)
	} else {
		print(Yellow+prefix+Reset, v...)
	}
}

func Fatal(v ...any) {
	prefix := "ERROR: "
	if NoColor() {
		print(prefix, v...)
	} else {
		print(Red+prefix+Reset, v...)
	}
	os.Exit(1)
}

func print(prefix string, v ...any) {
	fmt.Println(prefix, fmt.Sprint(v...), Reset)
}

func Progress(read, total int64, elapsed time.Duration) {
	mbRead := float64(read) / 1024 / 1024
	mbTot := float64(total) / 1024 / 1024
	speed := mbRead / elapsed.Seconds()

	prefix := "INFO: "
	if !NoColor() {
		prefix = Green + "INFO:" + Reset + " "
	}

	fmt.Printf(
		"\r%sDownloading… %.2f/%.2f MB (%.2f MB/s)%s",
		prefix, mbRead, mbTot, speed, Reset,
	)
}

func FinishProgress(read, total int64, elapsed time.Duration) {
	avgKB := float64(read) / 1024 / elapsed.Seconds()

	prefix := "INFO: "
	if !NoColor() {
		prefix = Green + "INFO:" + Reset + " "
	}

	fmt.Printf(
		" %sDownloaded %d/%d bytes (avg %.1f KB/s)%s\n",
		prefix, read, total, avgKB, Reset,
	)
}
