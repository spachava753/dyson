package xos

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
