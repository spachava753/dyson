package builtins

import (
	"errors"
	"io"
	"math"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/stdlibfs"
	"github.com/spf13/afero"
	"go.starlark.net/starlark"
)

type countingFile struct {
	afero.File
	data        string
	offset      int
	readCalls   int
	readBytes   int
	maxChunk    int
	terminalErr error
}

func (f *countingFile) Read(buffer []byte) (int, error) {
	f.readCalls++
	if f.offset == len(f.data) {
		if f.terminalErr != nil {
			return 0, f.terminalErr
		}
		return 0, io.EOF
	}
	if f.maxChunk > 0 && len(buffer) > f.maxChunk {
		buffer = buffer[:f.maxChunk]
	}
	n := copy(buffer, f.data[f.offset:])
	f.offset += n
	f.readBytes += n
	return n, nil
}

func (*countingFile) Close() error { return nil }

type countingFS struct {
	afero.Fs
	file *countingFile
}

func (f countingFS) Open(string) (afero.File, error) {
	return f.file, nil
}

func TestOpenRejectsDirectories(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		name string
		fsys afero.Fs
	}{
		{name: "host", fsys: stdlibfs.NewHost(afero.NewOsFs(), root)},
		{name: "io fs", fsys: stdlibfs.NewIOFS(fstest.MapFS{"file.txt": {Data: []byte("content")}})},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := NewFileRegistry(test.fsys)
			t.Cleanup(func() {
				be.Err(t, files.Close(), nil)
			})

			_, err := starlark.Call(
				&starlark.Thread{Name: "test"},
				files.Globals()["open"],
				starlark.Tuple{starlark.String(".")},
				nil,
			)
			be.Equal(t, errors.Is(err, syscall.EISDIR), true)
		})
	}
}

func TestBinaryReadRejectsSizeBelowNegativeOne(t *testing.T) {
	hostFile := &countingFile{data: "content"}
	file := openFile(
		t,
		hostFile,
		starlark.Tuple{starlark.String("content.bin")},
		[]starlark.Tuple{{starlark.String("mode"), starlark.String("rb")}},
	)
	read, err := file.Attr("read")
	be.Err(t, err, nil)

	_, err = starlark.Call(&starlark.Thread{Name: "test"}, read, starlark.Tuple{starlark.MakeInt(-2)}, nil)
	if err == nil || !strings.Contains(err.Error(), "read length must be non-negative or -1") {
		t.Fatalf("read(-2) error = %v, want invalid binary read length", err)
	}
	be.Equal(t, hostFile.readCalls, 0)

	value, err := starlark.Call(&starlark.Thread{Name: "test"}, read, starlark.Tuple{starlark.MakeInt(-1)}, nil)
	be.Err(t, err, nil)
	be.Equal(t, value, starlark.Value(starlark.Bytes("content")))
}

func TestSizedTextReadIsBounded(t *testing.T) {
	hostFile := &countingFile{data: strings.Repeat("a", 1<<20)}
	file := openFile(t, hostFile, starlark.Tuple{starlark.String("large.txt")}, nil)
	read, err := file.Attr("read")
	be.Err(t, err, nil)

	value, err := starlark.Call(&starlark.Thread{Name: "test"}, read, starlark.Tuple{starlark.MakeInt(0)}, nil)
	be.Err(t, err, nil)
	be.Equal(t, value, starlark.Value(starlark.String("")))
	be.Equal(t, hostFile.readCalls, 0)
	be.Equal(t, hostFile.readBytes, 0)

	value, err = starlark.Call(&starlark.Thread{Name: "test"}, read, starlark.Tuple{starlark.MakeInt(1)}, nil)
	be.Err(t, err, nil)
	be.Equal(t, value, starlark.Value(starlark.String("a")))
	if hostFile.readBytes > 4 {
		t.Fatalf("read(1) consumed %d bytes, want at most one UTF-8 character bound", hostFile.readBytes)
	}
}

func TestHugeSizedTextReadUsesCappedChunks(t *testing.T) {
	hostFile := &countingFile{data: "overflow-safe"}
	file := openFile(t, hostFile, starlark.Tuple{starlark.String("small.txt")}, nil)
	read, err := file.Attr("read")
	be.Err(t, err, nil)

	value, err := starlark.Call(&starlark.Thread{Name: "test"}, read, starlark.Tuple{starlark.MakeInt64(math.MaxInt64)}, nil)
	be.Err(t, err, nil)
	be.Equal(t, value, starlark.Value(starlark.String("overflow-safe")))
	be.Equal(t, hostFile.readCalls, 2)
}

func TestReadToEOFErrorPreservesBufferedCharacterCount(t *testing.T) {
	readErr := errors.New("read failed")
	hostFile := &countingFile{data: "buffered", terminalErr: readErr}
	file := openFile(t, hostFile, starlark.Tuple{starlark.String("partial.txt")}, nil)
	read, err := file.Attr("read")
	be.Err(t, err, nil)

	_, err = starlark.Call(&starlark.Thread{Name: "test"}, read, nil, nil)
	be.Equal(t, errors.Is(err, readErr), true)
	be.Equal(t, hostFile.readCalls, 2)

	value, err := starlark.Call(&starlark.Thread{Name: "test"}, read, starlark.Tuple{starlark.MakeInt(1)}, nil)
	be.Err(t, err, nil)
	be.Equal(t, value, starlark.Value(starlark.String("b")))
	be.Equal(t, hostFile.readCalls, 2)
}

func TestUniversalNewlinesCrossReadAndDecoderChunks(t *testing.T) {
	hostFile := &countingFile{data: "a\r\nb\rc", maxChunk: 1}
	file := openFile(t, hostFile, starlark.Tuple{starlark.String("lines.txt")}, nil)
	read, err := file.Attr("read")
	be.Err(t, err, nil)

	for _, want := range []string{"a\n", "b\n", "c"} {
		args := starlark.Tuple{starlark.MakeInt(2)}
		if want == "c" {
			args = nil
		}
		value, err := starlark.Call(&starlark.Thread{Name: "test"}, read, args, nil)
		be.Err(t, err, nil)
		be.Equal(t, value, starlark.Value(starlark.String(want)))
	}
}

func TestNewlineEmptyDisablesTranslation(t *testing.T) {
	hostFile := &countingFile{data: "a\r\nb\rc", maxChunk: 1}
	file := openFile(t, hostFile, starlark.Tuple{starlark.String("lines.txt")}, []starlark.Tuple{{starlark.String("newline"), starlark.String("")}})
	read, err := file.Attr("read")
	be.Err(t, err, nil)
	value, err := starlark.Call(&starlark.Thread{Name: "test"}, read, nil, nil)
	be.Err(t, err, nil)
	be.Equal(t, value, starlark.Value(starlark.String("a\r\nb\rc")))
}

func openFile(t *testing.T, hostFile *countingFile, args starlark.Tuple, kwargs []starlark.Tuple) starlark.HasAttrs {
	t.Helper()
	backing := afero.NewMemMapFs()
	be.Err(t, afero.WriteFile(backing, "fixture", []byte(hostFile.data), 0o600), nil)
	backingFile, err := backing.Open("fixture")
	be.Err(t, err, nil)
	hostFile.File = backingFile
	files := NewFileRegistry(countingFS{Fs: backing, file: hostFile})
	t.Cleanup(func() {
		be.Err(t, files.Close(), nil)
	})
	open := files.Globals()["open"]
	value, err := starlark.Call(&starlark.Thread{Name: "test"}, open, args, kwargs)
	be.Err(t, err, nil)
	file, ok := value.(starlark.HasAttrs)
	if !ok {
		t.Fatalf("open() returned %T, want file value", value)
	}
	return file
}
