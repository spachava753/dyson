package tempfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	"github.com/spachava753/dyson/internal/stdlibfs"
	"github.com/spachava753/dyson/internal/xos"
	"github.com/spf13/afero"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
	"go.starlark.net/syntax"
)

func TestTemporaryFileOpenFlags(t *testing.T) {
	for _, flag := range []int{os.O_RDWR, os.O_CREATE, os.O_EXCL} {
		if temporaryFileOpenFlags&flag != flag {
			t.Fatalf("temporary file flags %#x do not include %#x", temporaryFileOpenFlags, flag)
		}
	}
}

type lstatErrorFS struct {
	afero.Fs
	err error
}

func (f lstatErrorFS) LstatIfPossible(string) (os.FileInfo, bool, error) {
	return nil, true, f.err
}

func TestMktempPropagatesInspectionErrors(t *testing.T) {
	inspectionErr := errors.New("inspection failed")
	for _, test := range []struct {
		name string
		fsys afero.Fs
		want error
	}{
		{name: "stat fallback", fsys: afero.NewMemMapFs(), want: errors.ErrUnsupported},
		{name: "inspection failure", fsys: lstatErrorFS{Fs: afero.NewMemMapFs(), err: inspectionErr}, want: inspectionErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			module := MakeModule(ModuleConfig{FS: test.fsys})
			_, err := starlark.Call(
				&starlark.Thread{Name: "test"},
				module.Members["mktemp"],
				nil,
				[]starlark.Tuple{{starlark.String("dir"), starlark.String(".")}},
			)
			be.Equal(t, errors.Is(err, test.want), true)
		})
	}
}

func TestTempfileTestdata(t *testing.T) {
	t.Setenv("TMPDIR", ".")
	filename, err := filepath.Abs(filepath.Join("testdata", "tempfile.star"))
	be.Err(t, err, nil)

	fds := stdlibfs.NewFileDescriptors()
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
