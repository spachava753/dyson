package tempfile

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spachava753/dyson/internal/pybytes"
	"github.com/spachava753/dyson/internal/xfs"
	"github.com/spachava753/dyson/internal/xos"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's tempfile compatibility module.
const ModuleName = "tempfile"

const (
	tmpMax   = 10000
	template = "tmp"
)

// Module is the default fail-closed Starlark namespace exposed by
// load("tempfile.star", "tempfile"). Use MakeModule with filesystem and
// environment capabilities to allow temporary file creation.
var Module = MakeModule(ModuleConfig{})

// ModuleConfig groups the host domains used by Dyson's tempfile module.
type ModuleConfig struct {
	FS              xfs.FS
	Env             xos.Env
	FileDescriptors *xfs.FileDescriptors
}

// HostConfig returns a tempfile configuration backed by the host filesystem and OS.
func HostConfig(root string) ModuleConfig {
	return ModuleConfig{FS: xfs.HostFS{Root: root}, Env: xos.Host{}, FileDescriptors: xfs.NewFileDescriptors()}
}

// MakeModule returns a Starlark tempfile module backed by config.
func MakeModule(config ModuleConfig) *starlarkstruct.Module {
	if config.FileDescriptors == nil {
		config.FileDescriptors = xfs.NewFileDescriptors()
	}
	state := &moduleState{fsys: config.FS, env: config.Env, fds: config.FileDescriptors}
	m := &starlarkstruct.Module{
		Name: ModuleName,
		Members: starlark.StringDict{
			"NamedTemporaryFile":   starlark.NewBuiltin(ModuleName+".NamedTemporaryFile", state.namedTemporaryFile),
			"TemporaryFile":        starlark.NewBuiltin(ModuleName+".TemporaryFile", state.temporaryFile),
			"SpooledTemporaryFile": starlark.NewBuiltin(ModuleName+".SpooledTemporaryFile", unsupported("SpooledTemporaryFile")),
			"TemporaryDirectory":   starlark.NewBuiltin(ModuleName+".TemporaryDirectory", state.temporaryDirectory),
			"mkstemp":              starlark.NewBuiltin(ModuleName+".mkstemp", state.mkstemp),
			"mkdtemp":              starlark.NewBuiltin(ModuleName+".mkdtemp", state.mkdtemp),
			"mktemp":               starlark.NewBuiltin(ModuleName+".mktemp", state.mktemp),
			"gettempprefix":        starlark.NewBuiltin(ModuleName+".gettempprefix", gettempprefix),
			"gettempprefixb":       starlark.NewBuiltin(ModuleName+".gettempprefixb", gettempprefixb),
			"gettempdir":           starlark.NewBuiltin(ModuleName+".gettempdir", state.gettempdir),
			"gettempdirb":          starlark.NewBuiltin(ModuleName+".gettempdirb", state.gettempdirb),
			"TMP_MAX":              starlark.MakeInt(tmpMax),
			"template":             starlark.String(template),
			"tempdir":              starlark.None,
		},
	}
	m.Freeze()
	return m
}

type moduleState struct {
	fsys xfs.FS
	env  xos.Env
	fds  *xfs.FileDescriptors
	mu   sync.Mutex
	dir  string
}

func unsupported(name string) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return nil, fmt.Errorf("%s.%s: not supported", ModuleName, name)
	}
}

func gettempprefix(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.String(template), nil
}

func gettempprefixb(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return pybytes.NewString(template), nil
}

func (m *moduleState) gettempdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	dir, err := m.defaultTempDir(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.String(dir), nil
}

func (m *moduleState) gettempdirb(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	dir, err := m.defaultTempDir(fn.Name())
	if err != nil {
		return nil, err
	}
	return pybytes.NewString(dir), nil
}

func (m *moduleState) mkdtemp(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	params, err := m.params(fn.Name(), args, kwargs, false)
	if err != nil {
		return nil, err
	}
	fsys, err := m.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	for range tmpMax {
		name := joinPath(params.dir, params.prefix+randomName()+params.suffix)
		if err := fsys.Mkdir(name, 0o700); err == nil {
			return starlark.String(filepath.ToSlash(name)), nil
		} else if !isExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%s: no usable temporary directory name found", fn.Name())
}

func (m *moduleState) mkstemp(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	params, err := m.params(fn.Name(), args, kwargs, true)
	if err != nil {
		return nil, err
	}
	fsys, err := m.openfs(fn.Name())
	if err != nil {
		return nil, err
	}
	flag := params.flags
	if flag == 0 {
		flag = 0x242 // POSIX O_RDWR|O_CREAT|O_EXCL, matching xos.PortablePlatform.
	}
	for range tmpMax {
		name := joinPath(params.dir, params.prefix+randomName()+params.suffix)
		file, err := fsys.OpenFile(name, flag, 0o600)
		if err == nil {
			fd, err := m.fds.Store(file)
			if err != nil {
				return nil, errors.Join(fmt.Errorf("%s: %w", fn.Name(), err), file.Close())
			}
			return starlark.Tuple{starlark.MakeInt(fd), starlark.String(filepath.ToSlash(name))}, nil
		}
		if !isExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%s: no usable temporary file name found", fn.Name())
}

func (m *moduleState) mktemp(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var suffix, prefix string = "", template
	var dir starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "suffix?", &suffix, "prefix?", &prefix, "dir?", &dir); err != nil {
		return nil, err
	}
	dirname, err := m.optionalDir(fn.Name(), dir)
	if err != nil {
		return nil, err
	}
	for range tmpMax {
		name := joinPath(dirname, prefix+randomName()+suffix)
		if _, err := m.fsys.Lstat(name); err != nil {
			return starlark.String(filepath.ToSlash(name)), nil
		}
	}
	return nil, fmt.Errorf("%s: no usable temporary filename found", fn.Name())
}

func (m *moduleState) namedTemporaryFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return m.mkstemp(thread, fn, args, kwargs)
}

func (m *moduleState) temporaryFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	result, err := m.mkstemp(thread, fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	tuple := result.(starlark.Tuple)
	return tuple[0], nil
}

func (m *moduleState) temporaryDirectory(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return m.mkdtemp(thread, fn, args, kwargs)
}

type params struct {
	suffix string
	prefix string
	dir    string
	flags  int
}

func (m *moduleState) params(fn string, args starlark.Tuple, kwargs []starlark.Tuple, allowText bool) (params, error) {
	var suffix, prefix, dir starlark.Value = starlark.None, starlark.None, starlark.None
	text := false
	if allowText {
		if err := starlark.UnpackArgs(fn, args, kwargs, "suffix?", &suffix, "prefix?", &prefix, "dir?", &dir, "text?", &text); err != nil {
			return params{}, err
		}
	} else if err := starlark.UnpackArgs(fn, args, kwargs, "suffix?", &suffix, "prefix?", &prefix, "dir?", &dir); err != nil {
		return params{}, err
	}
	suf, err := optionalString(fn, "suffix", suffix, "")
	if err != nil {
		return params{}, err
	}
	pre, err := optionalString(fn, "prefix", prefix, template)
	if err != nil {
		return params{}, err
	}
	dirname, err := m.optionalDir(fn, dir)
	if err != nil {
		return params{}, err
	}
	flags := 0x242
	if text {
		flags = 0x242
	}
	return params{suffix: suf, prefix: pre, dir: dirname, flags: flags}, nil
}

func (m *moduleState) optionalDir(fn string, val starlark.Value) (string, error) {
	if val == starlark.None {
		return m.defaultTempDir(fn)
	}
	return optionalString(fn, "dir", val, "")
}

func optionalString(fn, name string, val starlark.Value, def string) (string, error) {
	if val == starlark.None {
		return def, nil
	}
	s, ok := starlark.AsString(val)
	if !ok {
		return "", fmt.Errorf("%s: %s must be a string or None", fn, name)
	}
	return s, nil
}

func (m *moduleState) defaultTempDir(fn string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dir != "" {
		return m.dir, nil
	}
	if m.fsys == nil {
		return "", fmt.Errorf("%s: filesystem operations are not configured", fn)
	}
	candidates := m.candidateDirs()
	for _, dir := range candidates {
		if info, err := m.fsys.Stat(dir); err == nil && info.IsDir() {
			m.dir = filepath.ToSlash(dir)
			return m.dir, nil
		}
	}
	return "", fmt.Errorf("%s: no usable temporary directory found", fn)
}

func (m *moduleState) candidateDirs() []string {
	var dirs []string
	if m.env != nil {
		for _, name := range []string{"TMPDIR", "TEMP", "TMP"} {
			if dir, ok := m.env.LookupEnv(name); ok && dir != "" {
				dirs = append(dirs, filepath.ToSlash(dir))
			}
		}
	}
	dirs = append(dirs, "/tmp", "/var/tmp", "/usr/tmp", ".")
	return dirs
}

func (m *moduleState) mut(fn string) (xfs.MutFS, error) {
	fsys, ok := m.fsys.(xfs.MutFS)
	if !ok {
		return nil, fmt.Errorf("%s: filesystem does not support mutation", fn)
	}
	return fsys, nil
}

func (m *moduleState) openfs(fn string) (xfs.OpenFS, error) {
	fsys, ok := m.fsys.(xfs.OpenFS)
	if !ok {
		return nil, fmt.Errorf("%s: filesystem does not support file descriptors", fn)
	}
	return fsys, nil
}

func randomName() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789_"
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		for i := range b {
			b[i] = byte(i)
		}
	}
	var n uint64 = binary.LittleEndian.Uint64(b[:])
	out := make([]byte, 8)
	for i := range out {
		out[i] = chars[n%uint64(len(chars))]
		n /= uint64(len(chars))
	}
	return string(out)
}

func joinPath(dir, name string) string {
	if dir == "" || dir == "." {
		return name
	}
	if strings.HasSuffix(dir, "/") {
		return dir + name
	}
	return dir + "/" + name
}

func isExist(err error) bool { return errors.Is(err, fs.ErrExist) }
