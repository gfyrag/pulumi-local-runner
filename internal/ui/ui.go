package ui

import (
	"fmt"

	"github.com/fatih/color"
)

var (
	header  = color.New(color.Bold, color.FgCyan)
	op      = color.New(color.Bold, color.FgMagenta)
	success = color.New(color.FgGreen)
	warn    = color.New(color.FgYellow)
	fail    = color.New(color.FgRed, color.Bold)
	info    = color.New(color.FgBlue)
	dim     = color.New(color.Faint)
)

// Header prints a prominent section header: === app/stack [op] ===
func Header(target, operation string) {
	fmt.Println()
	header.Printf("=== %s ", target)
	op.Printf("[%s]", operation)
	header.Println(" ===")
}

// Info prints an informational message.
func Info(format string, a ...any) {
	info.Printf("  → "+format+"\n", a...)
}

// Step prints a step being executed.
func Step(format string, a ...any) {
	dim.Printf("  "+format+"\n", a...)
}

// Warn prints a warning message.
func Warn(format string, a ...any) {
	warn.Printf("  ⚠ "+format+"\n", a...)
}

// Result prints a summary line for a target result.
func ResultOK(target string) {
	success.Printf("  ✓ %s\n", target)
}

func ResultFail(target, msg string) {
	fail.Printf("  ✗ %s: %s\n", target, msg)
}

// Summary prints a final summary.
func Summary(total, succeeded, failed int) {
	fmt.Println()
	if failed == 0 {
		success.Printf("All %d target(s) succeeded.\n", total)
	} else {
		fail.Printf("%d/%d target(s) failed.\n", failed, total)
	}
}
