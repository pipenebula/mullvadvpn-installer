package ui

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

type UI struct {
	In        *bufio.Reader
	Out, Err  io.Writer
	AssumeYes bool
	DryRun    bool
	NoColor   bool
}

func NewUI(r io.Reader, w, e io.Writer, assumeYes, dryRun, noColor bool) *UI {
	return &UI{
		In:        bufio.NewReader(r),
		Out:       w,
		Err:       e,
		AssumeYes: assumeYes,
		DryRun:    dryRun,
		NoColor:   noColor,
	}
}

type Step func(u *UI) error

func (u *UI) RunAll(steps ...Step) error {
	for _, step := range steps {
		if err := step(u); err != nil {
			return err
		}
	}
	return nil
}

func LogStep(f func(v ...any)) Step {
	return func(_ *UI) error {
		f()
		return nil
	}
}

func (u *UI) printLine(s string) { fmt.Fprintln(u.Out, s) }
func (u *UI) print(s string)     { fmt.Fprint(u.Out, s) }

func (u *UI) prompt(label, def string) (string, error) {
	u.print(label)
	line, err := u.In.ReadString('\n')
	if err != nil {
		return "", err
	}
	ans := strings.TrimSpace(strings.ToLower(line))
	if ans == "" {
		return def, nil
	}
	return ans, nil
}

func Confirm(msg string, def bool, onResult func(ok bool)) Step {
	return func(u *UI) error {
		if u.AssumeYes || u.DryRun {
			onResult(true)
			return nil
		}
		opts := "[y/N]"
		if def {
			opts = "[Y/n]"
		}
		for {
			var label string
			if u.NoColor {
				label = fmt.Sprintf("%s %s ", msg, opts)
			} else {
				label = fmt.Sprintf("%s%s %s %s", Pink, msg, opts, Reset)
			}
			ans, err := u.prompt(label, "")
			if err != nil {
				return err
			}
			switch ans {
			case "y", "yes":
				onResult(true)
				return nil
			case "n", "no":
				onResult(false)
				return nil
			case "":
				onResult(def)
				return nil
			default:
				if u.NoColor {
					u.printLine(MsgInvalidYesNo)
				} else {
					u.printLine(Red + MsgInvalidYesNo + Reset)
				}
			}
		}
	}
}

func ConfirmfLazy(
	format string,
	def bool,
	onResult func(ok bool),
	argFns ...func() any,
) Step {
	return func(u *UI) error {
		vals := make([]any, len(argFns))
		for i, fn := range argFns {
			vals[i] = fn()
		}
		msg := fmt.Sprintf(format, vals...)
		return Confirm(msg, def, onResult)(u)
	}
}

type Conditional struct {
	Cond func() bool
	S    Step
}

func (c Conditional) Run(u *UI) error {
	if !c.Cond() {
		return nil
	}
	return c.S(u)
}

func Spinner(msg string, count int, delay time.Duration) Step {
	return func(u *UI) error {
		if u.NoColor {
			u.print(msg)
		} else {
			u.print(Blue + msg + Reset)
		}
		for i := 0; i < count; i++ {
			if u.NoColor {
				u.print(".")
			} else {
				u.print(Blue + "." + Reset)
			}
			time.Sleep(delay)
		}
		u.printLine("")
		return nil
	}
}

func SelectTwo(
	msg string,
	opt1, opt2 string,
	defFirst bool,
	onResult func(chosenFirst bool),
) Step {
	return func(u *UI) error {
		if u.AssumeYes || u.DryRun {
			onResult(defFirst)
			return nil
		}
		if u.NoColor {
			u.printLine(msg)
		} else {
			u.printLine(Pink + msg + Reset)
		}
		u.printLine("  1) " + opt1)
		u.printLine("  2) " + opt2)

		var label string
		if u.NoColor {
			label = fmt.Sprintf("Choice [1]: ")
		} else {
			label = fmt.Sprintf("%sChoice [1]: %s", Pink, Reset)
		}

		for {
			ans, err := u.prompt(label, "")
			if err != nil {
				return err
			}
			switch strings.TrimSpace(ans) {
			case "", "1":
				onResult(defFirst)
				return nil
			case "2":
				onResult(!defFirst)
				return nil
			default:
				if u.NoColor {
					u.printLine(MsgInvalidChoice)
				} else {
					u.printLine(Red + MsgInvalidChoice + Reset)
				}
			}
		}
	}
}

const (
	OptGoXZ     = "Built-in parsers (Go AR + Go XZ decompressor; portable, slower)"
	OptSystemXZ = "System utilities (ar+xz; high-performance, requires binutils and xz installed)"
)

func SelectStableBeta(msg string, onResult func(string)) Step {
	return SelectTwo(msg, OptStable, OptBeta, true, func(first bool) {
		if first {
			onResult(OptStable)
		} else {
			onResult(OptBeta)
		}
	})
}

func SelectXZBackend(msg string, onResult func(useSystem bool)) Step {
	return SelectTwo(msg, OptGoXZ, OptSystemXZ, true, func(first bool) {
		onResult(!first)
	})
}
