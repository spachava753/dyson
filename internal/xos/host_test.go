package xos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestHostRunCommandUsesEnvPathForLookup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell script")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "dyson-xos-test")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf from-env\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := (Host{}).RunCommand(t.Context(), Command{
		Args:   []string{"dyson-xos-test"},
		Env:    []string{"PATH=" + dir},
		Stdout: StreamPipe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Stdout) != "from-env" {
		t.Fatalf("stdout = %q, want %q", string(result.Stdout), "from-env")
	}
}

func TestHostRunCommandIgnoresStdinBrokenPipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses the POSIX true command")
	}
	for range 5 {
		result, err := (Host{}).RunCommand(t.Context(), Command{
			Args:  []string{"true"},
			Input: bytes.Repeat([]byte("x"), 1<<20),
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.ReturnCode != 0 {
			t.Fatalf("return code = %d, want 0", result.ReturnCode)
		}
	}
}

func TestHostRunCommandUsesPipeForEmptyStdin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell command")
	}
	result, err := (Host{}).RunCommand(t.Context(), Command{
		Args:  []string{"test -p /dev/stdin"},
		Shell: true,
		Stdin: StreamPipe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ReturnCode != 0 {
		t.Fatalf("return code = %d, stdin is not a pipe", result.ReturnCode)
	}
}

func TestHostRunCommandWaitsForCapturedPipesWithoutCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell command")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	start := time.Now()
	_, err := (Host{}).RunCommand(ctx, Command{
		Args:   []string{`trap '' HUP; sleep 0.25 &`},
		Shell:  true,
		Stdout: StreamPipe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Fatalf("RunCommand returned after %v before descendant closed stdout", elapsed)
	}
}

func TestHostRunCommandClosesCapturedPipesAfterCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell command")
	}
	pidFile := filepath.Join(t.TempDir(), "descendant.pid")
	type outcome struct {
		result CommandResult
		err    error
	}
	ctx, cancel := context.WithCancel(t.Context())
	outcomes := make(chan outcome, 1)
	go func() {
		result, err := (Host{}).RunCommand(ctx, Command{
			Args:   []string{`trap '' HUP; printf 'partial stdout'; printf 'partial stderr' >&2; sleep 0.05; sleep 30 & child=$!; printf '%s\n' "$child" > "$PID_FILE"; wait`},
			Shell:  true,
			Env:    append(os.Environ(), "PID_FILE="+pidFile),
			Stdout: StreamPipe,
			Stderr: StreamPipe,
		})
		outcomes <- outcome{result: result, err: err}
	}()

	pid, err := waitForPID(pidFile, 2*time.Second)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	if err := syscall.Kill(pid, 0); err != nil {
		cancel()
		t.Fatalf("descendant %d is not running: %v", pid, err)
	}

	start := time.Now()
	cancel()
	select {
	case outcome := <-outcomes:
		if !errors.Is(outcome.err, context.Canceled) {
			t.Fatalf("RunCommand error = %v, want context canceled", outcome.err)
		}
		if got := string(outcome.result.Stdout); got != "partial stdout" {
			t.Fatalf("stdout = %q, want %q", got, "partial stdout")
		}
		if got := string(outcome.result.Stderr); got != "partial stderr" {
			t.Fatalf("stderr = %q, want %q", got, "partial stderr")
		}
	case <-time.After(time.Second):
		t.Fatal("RunCommand did not close captured pipes after cancellation")
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Fatalf("RunCommand returned after %v; descendant pipe drain was not canceled", elapsed)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("kill descendant %d: %v", pid, err)
	}
}

func TestHostRunCommandClosesStdinAfterCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell command")
	}
	pidFile := filepath.Join(t.TempDir(), "stdin-descendant.pid")
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() {
		_, err := (Host{}).RunCommand(ctx, Command{
			Args:  []string{`trap '' HUP; sleep 30 <&0 & child=$!; printf '%s\n' "$child" > "$PID_FILE"; wait`},
			Shell: true,
			Input: bytes.Repeat([]byte("x"), 1<<20),
			Env:   append(os.Environ(), "PID_FILE="+pidFile),
		})
		result <- err
	}()

	pid, err := waitForPID(pidFile, 2*time.Second)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	if err := syscall.Kill(pid, 0); err != nil {
		cancel()
		t.Fatalf("descendant %d is not running: %v", pid, err)
	}
	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-result:
		cancel()
		t.Fatalf("RunCommand returned before cancellation: %v", err)
	default:
	}

	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunCommand error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunCommand did not close stdin after cancellation")
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("kill descendant %d: %v", pid, err)
	}
}

func waitForPID(path string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				return 0, fmt.Errorf("parse descendant PID: %w", err)
			}
			return pid, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("timed out waiting for descendant PID")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
