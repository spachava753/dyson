package tempfile

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	"github.com/spachava753/dyson/internal/xfs"
	"github.com/spachava753/dyson/internal/xos"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
	"go.starlark.net/syntax"
)

func TestTempfileTestdata(t *testing.T) {
	t.Setenv("TMPDIR", ".")
	filename, err := filepath.Abs(filepath.Join("testdata", "tempfile.star"))
	be.Err(t, err, nil)

	fds := xfs.NewFileDescriptors()
	root := t.TempDir()
	osConfig := stdlibos.HostConfig(root)
	osConfig.FileDescriptors = fds
	tempfileConfig := HostConfig(root)
	tempfileConfig.Env = xos.Host{}
	tempfileConfig.FileDescriptors = fds

	for i, chunk := range chunkedfile.Read(filename, t) {
		t.Run(fmt.Sprintf("chunk_%02d", i+1), func(t *testing.T) {
			thread := newTestThread(t, stdlibos.MakeModule(osConfig), MakeModule(tempfileConfig))
			_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, filename, chunk.Source, nil)
			if err != nil {
				chunk.GotErrorAnyLine(err.Error())
			}
			chunk.Done()
		})
	}
}

func newTestThread(t *testing.T, osModule, tempfileModule starlark.Value) *starlark.Thread {
	thread := &starlark.Thread{
		Name: "tempfile_test",
		Load: func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
			switch module {
			case "assert.star":
				return starlarktest.LoadAssertModule()
			case stdlibos.ModuleName + ".star":
				return starlark.StringDict{stdlibos.ModuleName: osModule}, nil
			case ModuleName + ".star":
				return starlark.StringDict{ModuleName: tempfileModule}, nil
			default:
				return nil, fmt.Errorf("unknown module %q", module)
			}
		},
	}
	starlarktest.SetReporter(thread, t)
	return thread
}
