//go:build mage

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var (
	// Default target executed when none is specified.
	Default = CI
)

// CI runs the standard Phase 1 pipeline: format, lint, test, build.
func CI() {
	mg.SerialDeps(Format, Lint, Test, Build)
}

// Format updates Go sources using gofmt.
func Format() error {
	return run("go", "fmt", "./...")
}

// Lint executes go vet to perform static analysis.
func Lint() error {
	return run("go", "vet", "./...")
}

// Test runs the full Go test suite with race detection disabled (Phase 1 scope).
func Test() error {
	return run("go", "test", "./...")
}

// Build compiles all packages to verify build correctness.
func Build() error {
	if err := run("go", "build", "./..."); err != nil {
		return err
	}

	version := resolveVersion()
	ldflags := fmt.Sprintf("-X github.com/bkyoung/code-reviewer/internal/version.version=%s", version)
	return run("go", "build", "-ldflags", ldflags, "-o", "cr", "./cmd/cr")
}

func run(cmd string, args ...string) error {
	if err := sh.RunV(cmd, args...); err != nil {
		return fmt.Errorf("%s %v: %w", cmd, args, err)
	}
	return nil
}

func resolveVersion() string {
	const defaultVersion = "v0.0.0"

	tag, err := gitOutput("describe", "--tags", "--abbrev=0")
	if err != nil {
		return defaultVersion
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return defaultVersion
	}

	if repoDirty() {
		return tag + "-dirty"
	}

	if !headMatchesTag() {
		return tag + "-dirty"
	}

	return tag
}

func repoDirty() bool {
	output, err := gitOutput("status", "--porcelain")
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) != ""
}

func headMatchesTag() bool {
	_, err := gitOutput("describe", "--tags", "--exact-match")
	if err != nil {
		errText := err.Error()
		switch {
		case strings.Contains(errText, "no tag exactly matches"),
			strings.Contains(errText, "no names found"):
			return false
		default:
			return false
		}
	}
	return true
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			err = fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return stdout.String(), nil
}
