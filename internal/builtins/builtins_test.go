package builtins

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
	"go.starlark.net/syntax"
)

// TestBuiltinsTestdata executes each Starlark testdata chunk with Dyson's
// Python-like builtins predeclared in the global namespace.
func TestBuiltinsTestdata(t *testing.T) {
	filename := filepath.Join("testdata", "builtins.star")
	for i, chunk := range chunkedfile.Read(filename, t) {
		t.Run(fmt.Sprintf("chunk_%02d", i+1), func(t *testing.T) {
			thread := newTestThread(t)
			predeclared := make(starlark.StringDict, len(builtinsFunc)+4)
			maps.Copy(predeclared, builtinsFunc)
			maps.Copy(predeclared, testPathGlobals(t))
			_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, filename, chunk.Source, predeclared)
			if err != nil {
				chunk.GotErrorAnyLine(err.Error())
			}
			chunk.Done()
		})
	}
}

// testPathGlobals creates per-test filesystem fixtures used by open tests and
// returns their paths as Starlark globals.
func testPathGlobals(t *testing.T) starlark.StringDict {
	t.Helper()
	dir := t.TempDir()
	absolute := filepath.Join(dir, "absolute.txt")
	be.Err(t, os.WriteFile(absolute, []byte("absolute file\n"), 0o600), nil)
	binary := filepath.Join(dir, "binary.bin")
	be.Err(t, os.WriteFile(binary, []byte{0x00, 0x01, 0x02, 0xff}, 0o600), nil)
	relativeDir := filepath.Join(dir, "cwd")
	be.Err(t, os.Mkdir(relativeDir, 0o700), nil)
	relativeFile := filepath.Join(relativeDir, "relative.txt")
	be.Err(t, os.WriteFile(relativeFile, []byte("relative file\n"), 0o600), nil)
	oldwd, err := os.Getwd()
	be.Err(t, err, nil)
	be.Err(t, os.Chdir(relativeDir), nil)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	return starlark.StringDict{
		"TEST_OPEN_RELATIVE":  starlark.String("relative.txt"),
		"TEST_OPEN_ABSOLUTE":  starlark.String(absolute),
		"TEST_OPEN_BINARY":    starlark.String(binary),
		"TEST_OPEN_MALFORMED": starlark.String("bad\x00path"),
	}
}

// newTestThread returns a Starlark thread configured with assert.star for the
// chunked builtins testdata.
func newTestThread(t *testing.T) *starlark.Thread {
	thread := &starlark.Thread{
		Name: "test",
		Load: func(thread *starlark.Thread, name string) (starlark.StringDict, error) {
			switch name {
			case "assert.star":
				return starlarktest.LoadAssertModule()
			default:
				return nil, fmt.Errorf("unknown module %q", name)
			}
		},
	}
	starlarktest.SetReporter(thread, t)
	return thread
}
