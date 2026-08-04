package dyson_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson"
	"github.com/spf13/afero"
	"go.starlark.net/starlark"
)

type trackedReadFile struct {
	afero.File
	closeCalls int
	closeErr   error
}

func (f *trackedReadFile) Close() error {
	f.closeCalls++
	return errors.Join(f.closeErr, f.File.Close())
}

type trackingReadFS struct {
	afero.Fs
	files       []*trackedReadFile
	closeErrors map[string]error
}

func (f *trackingReadFS) Open(name string) (afero.File, error) {
	backing, err := f.prepare(name)
	if err != nil {
		return nil, err
	}
	file, err := backing.Open(name)
	if err != nil {
		return nil, err
	}
	return f.track(name, file), nil
}

func (f *trackingReadFS) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	backing, err := f.prepare(name)
	if err != nil {
		return nil, err
	}
	file, err := backing.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return f.track(name, file), nil
}

func (f *trackingReadFS) prepare(name string) (afero.Fs, error) {
	if f.Fs == nil {
		f.Fs = afero.NewMemMapFs()
	}
	if err := afero.WriteFile(f.Fs, name, []byte(name), 0o600); err != nil {
		return nil, err
	}
	return f.Fs, nil
}

func (f *trackingReadFS) track(name string, file afero.File) afero.File {
	tracked := &trackedReadFile{File: file, closeErr: f.closeErrors[name]}
	f.files = append(f.files, tracked)
	return tracked
}

func TestOpenRejectsDirectory(t *testing.T) {
	root := t.TempDir()
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root)))
	t.Cleanup(func() {
		be.Err(t, sphere.Close(), nil)
	})

	err := sphere.Eval(t.Context(), `open(".")`)
	be.Equal(t, errors.Is(err, syscall.EISDIR), true)
}

func TestOpenRejectsEmptyFilenameBeforeFilesystem(t *testing.T) {
	fsys := &trackingReadFS{}
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{FS: fsys}))
	t.Cleanup(func() {
		be.Err(t, sphere.Close(), nil)
	})

	err := sphere.Eval(t.Context(), `open("")`)
	be.Equal(t, errors.Is(err, fs.ErrNotExist), true)
	be.Equal(t, len(fsys.files), 0)
}

func TestOpenReadsTextAndBinaryFiles(t *testing.T) {
	root := t.TempDir()
	be.Err(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte{'h', 0xc3, 0xa9, 'l', 'l', 'o'}, 0o600), nil)
	be.Err(t, os.WriteFile(filepath.Join(root, "latin.txt"), []byte{'c', 'a', 'f', 0xe9}, 0o600), nil)
	be.Err(t, os.WriteFile(filepath.Join(root, "newlines.txt"), []byte("a\r\nb\rc"), 0o600), nil)
	be.Err(t, os.WriteFile(filepath.Join(root, "image.bin"), []byte{0x00, 0x01, 0xfe, 0xff}, 0o600), nil)
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root)))

	be.Err(t, sphere.Eval(t.Context(), `
text_file = open("notes.txt", mode="r", encoding="utf-8")
if text_file.read(2) != "h\u00e9":
    fail("text read(size) did not count decoded characters")
if text_file.read() != "llo":
    fail("text read did not return the remaining text")
text_file.close()

all_text_file = open("notes.txt")
if all_text_file.read() != "h\u00e9llo":
    fail("default text read failed")
all_text_file.close()

negative_text_file = open("notes.txt")
if negative_text_file.read(-2) != "h\u00e9llo":
    fail("negative text size did not read to EOF")
negative_text_file.close()

none_size_file = open("notes.txt")
if none_size_file.read(None) != "h\u00e9llo":
    fail("None size did not read to EOF")
none_size_file.close()

latin_file = open("latin.txt", encoding="latin-1")
if latin_file.read() != "caf\u00e9":
    fail("explicit text encoding failed")
latin_file.close()

replacement_file = open("latin.txt", errors="replace")
if replacement_file.read() != "caf\ufffd":
    fail("text error handler failed")
replacement_file.close()

ordered_file = open("notes.txt", "rt", -1, "utf-8", "strict", None, True, None)
if ordered_file.read() != "h\u00e9llo":
    fail("Python positional open contract failed")
ordered_file.close()

newline_file = open("newlines.txt")
if newline_file.read(2) != "a\n" or newline_file.read() != "b\nc":
    fail("universal-newline translation failed")
newline_file.close()

binary_file = open("image.bin", mode="rb")
binary_prefix = binary_file.read(2)
binary = binary_file.read()
binary_file.close()
if type(binary) != "bytes" or binary_prefix != b"\x00\x01" or binary != b"\xfe\xff":
    fail("binary read did not return bytes")

negative_one_binary_file = open("image.bin", mode="rb")
if negative_one_binary_file.read(-1) != b"\x00\x01\xfe\xff":
    fail("binary read(-1) did not read to EOF")
negative_one_binary_file.close()
`), nil)
}

func TestOpenFilePersistsAcrossEvalCallsUntilClosed(t *testing.T) {
	root := t.TempDir()
	be.Err(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("persistent"), 0o600), nil)
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root)))

	be.Err(t, sphere.Eval(t.Context(), `
file = open("notes.txt")
prefix = file.read(3)
`), nil)
	be.Err(t, sphere.Eval(t.Context(), `
if prefix != "per" or file.read() != "sistent":
    fail("file did not persist across evaluations")
file.close()
file.close()
`), nil)

	err := sphere.Eval(t.Context(), `file.read()`)
	if err == nil || !strings.Contains(err.Error(), "file.read") || !strings.Contains(err.Error(), "notes.txt") || !strings.Contains(err.Error(), "closed file") {
		t.Fatalf("Eval() error = %v, want closed-file operation and path context", err)
	}
}

func TestSphereCloseOwnsOpenFilesAndDescriptors(t *testing.T) {
	fsys := &trackingReadFS{}
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{FS: fsys}))

	err := sphere.Eval(t.Context(), `
load("os.star", "os")
explicit = open("explicit.txt")
explicit.close()
open("abandoned.txt")
explicit_fd = os.open("explicit-fd.txt", os.O_RDONLY)
os.close(explicit_fd)
os.open("abandoned-fd.txt", os.O_RDONLY)
fail("after opening")
`)
	if err == nil || !strings.Contains(err.Error(), "after opening") {
		t.Fatalf("Eval() error = %v, want failure after opening files", err)
	}
	be.Equal(t, len(fsys.files), 4)
	be.Equal(t, fsys.files[0].closeCalls, 1)
	be.Equal(t, fsys.files[1].closeCalls, 0)
	be.Equal(t, fsys.files[2].closeCalls, 1)
	be.Equal(t, fsys.files[3].closeCalls, 0)

	be.Err(t, sphere.Close(), nil)
	for _, file := range fsys.files {
		be.Equal(t, file.closeCalls, 1)
	}

	be.Err(t, sphere.Close(), nil)
	for _, file := range fsys.files {
		be.Equal(t, file.closeCalls, 1)
	}

	err = sphere.Eval(t.Context(), `open("too-late.txt")`)
	if err == nil || !strings.Contains(err.Error(), "sphere is closed") {
		t.Fatalf("Eval() error = %v, want closed-sphere error", err)
	}
	be.Equal(t, len(fsys.files), 4)
}

func TestStdlibOwnerClosesCustomLoaderFiles(t *testing.T) {
	fsys := &trackingReadFS{}
	stdlib := dyson.NewStdlib(dyson.StdlibConfig{FS: fsys})
	modules := stdlib.Modules()
	thread := &starlark.Thread{
		Name: "test",
		Load: func(_ *starlark.Thread, module string) (starlark.StringDict, error) {
			return modules[module], nil
		},
	}

	_, err := starlark.ExecFile(thread, "test.star", `
load("os.star", "os")
open("abandoned.txt")
os.open("abandoned-fd.txt", os.O_RDONLY)
fail("after opening")
`, stdlib.Globals())
	if err == nil || !strings.Contains(err.Error(), "after opening") {
		t.Fatalf("ExecFile() error = %v, want failure after opening files", err)
	}
	be.Equal(t, len(fsys.files), 2)
	be.Equal(t, fsys.files[0].closeCalls, 0)
	be.Equal(t, fsys.files[1].closeCalls, 0)

	be.Err(t, stdlib.Close(), nil)
	be.Equal(t, fsys.files[0].closeCalls, 1)
	be.Equal(t, fsys.files[1].closeCalls, 1)
	be.Err(t, stdlib.Close(), nil)
}

func TestSphereCloseContinuesAfterFileCloseError(t *testing.T) {
	globalCloseErr := errors.New("global close failed")
	descriptorCloseErr := errors.New("descriptor close failed")
	fsys := &trackingReadFS{closeErrors: map[string]error{
		"broken.txt":    globalCloseErr,
		"broken-fd.txt": descriptorCloseErr,
	}}
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{FS: fsys}))

	be.Err(t, sphere.Eval(t.Context(), `
load("os.star", "os")
first = open("broken.txt")
second = open("healthy.txt")
os.open("broken-fd.txt", os.O_RDONLY)
`), nil)
	err := sphere.Close()
	if !errors.Is(err, globalCloseErr) || !errors.Is(err, descriptorCloseErr) || !strings.Contains(err.Error(), "broken.txt") {
		t.Fatalf("Close() error = %v, want global and descriptor close errors", err)
	}
	for _, file := range fsys.files {
		be.Equal(t, file.closeCalls, 1)
	}
	be.Err(t, sphere.Close(), nil)
}

func TestOpenUsesExpandedHostPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	be.Err(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("expanded"), 0o600), nil)
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root)))

	be.Err(t, sphere.Eval(t.Context(), `
load("os.star", "os")
path = os.path.expanduser("~/notes.txt")
file = open(path)
if file.read() != "expanded":
    fail("open and os.path did not share path behavior")
file.close()
`), nil)
}

func TestOpenErrorsIncludeOperationAndPath(t *testing.T) {
	root := t.TempDir()
	be.Err(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("notes"), 0o600), nil)
	for _, test := range []struct {
		name   string
		sphere *dyson.Sphere
		code   string
		want   string
	}{
		{name: "filesystem unavailable", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{})), code: `open("secret.txt")`, want: `open("secret.txt"): filesystem access is not configured`},
		{name: "missing file", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("missing.txt")`, want: `open("missing.txt")`},
		{name: "invalid mode", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("notes.txt", mode="w")`, want: `open("notes.txt"): unsupported mode "w"`},
		{name: "third positional is buffering", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("notes.txt", "r", "utf-8")`, want: `for parameter buffering`},
		{name: "unsupported buffering", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("notes.txt", buffering=0)`, want: `buffering values other than -1 are not supported`},
		{name: "unsupported encoding", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("notes.txt", encoding="utf-16")`, want: `open("notes.txt"): unsupported encoding "utf-16"`},
		{name: "unsupported errors", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("notes.txt", errors="backslashreplace")`, want: `unsupported error handler "backslashreplace"`},
		{name: "illegal newline", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("notes.txt", newline="invalid")`, want: `illegal newline value "invalid"`},
		{name: "unsupported closefd", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("notes.txt", closefd=False)`, want: `closefd=False is not supported`},
		{name: "unsupported opener", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("notes.txt", opener=True)`, want: `opener is not supported`},
		{name: "binary encoding", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("notes.txt", mode="rb", encoding="utf-8")`, want: `open("notes.txt"): binary mode does not take an encoding argument`},
		{name: "binary size below negative one", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("notes.txt", mode="rb").read(-2)`, want: `read length must be non-negative or -1`},
		{name: "read size is positional only", sphere: dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root))), code: `open("notes.txt").read(size=1)`, want: `file.read: unexpected keyword arguments`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Cleanup(func() {
				be.Err(t, test.sphere.Close(), nil)
			})
			err := test.sphere.Eval(t.Context(), test.code)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Eval() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestOSReadBytesSupportDecode(t *testing.T) {
	root := t.TempDir()
	be.Err(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte{'c', 'a', 'f', 0xe9}, 0o600), nil)
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root)))

	be.Err(t, sphere.Eval(t.Context(), `
load("os.star", "os")
fd = os.open("notes.txt", os.O_RDONLY)
data = os.read(fd, 100000)
os.close(fd)
if data.decode("latin-1") != "caf\u00e9":
    fail("latin-1 decode failed")
if data.decode("ascii", "ignore") != "caf":
    fail("ascii ignore decode failed")
if data.decode(encoding="ascii", errors="replace") != "caf\ufffd":
    fail("ascii replacement decode failed")
`), nil)
}
