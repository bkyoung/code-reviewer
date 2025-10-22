package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bkyoung/code-reviewer/internal/adapter/cli"
	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

type branchStub struct {
	request review.BranchRequest
	err     error
	current string
}

func (b *branchStub) ReviewBranch(ctx context.Context, req review.BranchRequest) (review.Result, error) {
	b.request = req
	return review.Result{}, b.err
}

func (b *branchStub) CurrentBranch(ctx context.Context) (string, error) {
	if b.current == "" {
		return "", errors.New("no branch")
	}
	return b.current, nil
}

func TestReviewBranchCommandInvokesUseCase(t *testing.T) {
	stub := &branchStub{}
	root := cli.NewRootCommand(cli.Dependencies{
		BranchReviewer: stub,
		Args:           cli.Arguments{OutWriter: io.Discard, ErrWriter: io.Discard},
		DefaultOutput:  "build",
		DefaultRepo:    "demo",
		Version:        "v1.2.3",
	})

	root.SetArgs([]string{"review", "branch", "feature", "--base", "master", "--include-uncommitted"})
	if err := root.Execute(); err != nil {
		t.Fatalf("command execution failed: %v", err)
	}

	if stub.request.TargetRef != "feature" {
		t.Fatalf("expected target ref feature, got %s", stub.request.TargetRef)
	}

	if stub.request.OutputDir != "build" {
		t.Fatalf("expected default output dir build, got %s", stub.request.OutputDir)
	}

	if !stub.request.IncludeUncommitted {
		t.Fatalf("expected include uncommitted to be true")
	}
}

func TestReviewBranchCommandDetectsTarget(t *testing.T) {
	stub := &branchStub{current: "detected"}
	root := cli.NewRootCommand(cli.Dependencies{
		BranchReviewer: stub,
		Args:           cli.Arguments{OutWriter: io.Discard, ErrWriter: io.Discard},
		DefaultOutput:  "out",
		DefaultRepo:    "demo",
		Version:        "v1.2.3",
	})

	root.SetArgs([]string{"review", "branch", "--base", "master", "--detect-target"})
	if err := root.Execute(); err != nil {
		t.Fatalf("command execution failed: %v", err)
	}

	if stub.request.TargetRef != "detected" {
		t.Fatalf("expected target ref detected, got %s", stub.request.TargetRef)
	}
}

func TestVersionFlagEmitsVersion(t *testing.T) {
	stub := &branchStub{}
	buf := &bytes.Buffer{}
	root := cli.NewRootCommand(cli.Dependencies{
		BranchReviewer: stub,
		Args:           cli.Arguments{OutWriter: buf, ErrWriter: io.Discard},
		Version:        "v9.9.9",
	})

	root.SetArgs([]string{"--version"})
	err := root.Execute()
	if !errors.Is(err, cli.ErrVersionRequested) {
		t.Fatalf("expected version sentinel, got %v", err)
	}
	if strings.TrimSpace(buf.String()) != "v9.9.9" {
		t.Fatalf("unexpected version output: %q", buf.String())
	}
}
