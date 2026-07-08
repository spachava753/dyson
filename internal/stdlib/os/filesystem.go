package os

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/spachava753/dyson/internal/xfs"
	"github.com/spachava753/dyson/internal/xos"
	"go.starlark.net/starlark"
)

type FileSystem struct {
	fsys     xfs.FS
	platform xos.Platform
	fds      *xfs.FileDescriptors
}

var dirEntryMethods = map[string]*starlark.Builtin{
	"is_dir":  starlark.NewBuiltin("os.DirEntry.is_dir", dirEntryIsDir),
	"is_file": starlark.NewBuiltin("os.DirEntry.is_file", dirEntryIsFile),
	"stat":    starlark.NewBuiltin("os.DirEntry.stat", dirEntryStat),
}

func (f FileSystem) mut(fn string) (xfs.MutFS, error) {
	fsys, ok := f.fsys.(xfs.MutFS)
	if !ok {
		return nil, fmt.Errorf("%s: filesystem does not support mutation", fn)
	}
	return fsys, nil
}

func (f FileSystem) openfs(fn string) (xfs.OpenFS, error) {
	fsys, ok := f.fsys.(xfs.OpenFS)
	if !ok {
		return nil, fmt.Errorf("%s: filesystem does not support file descriptors", fn)
	}
	return fsys, nil
}

func (f FileSystem) pathfs(fn string) (xfs.PathFS, error) {
	fsys, ok := f.fsys.(xfs.PathFS)
	if !ok {
		return nil, fmt.Errorf("%s: filesystem does not support host path resolution", fn)
	}
	return fsys, nil
}

func (f FileSystem) samefile(fn string) (xfs.SameFileFS, error) {
	fsys, ok := f.fsys.(xfs.SameFileFS)
	if !ok {
		return nil, fmt.Errorf("%s: filesystem does not support path identity checks", fn)
	}
	return fsys, nil
}

func none(fn string, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn, args, kwargs); err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func intArg(fn, name string, value starlark.Int) (int, error) {
	n, ok := value.Int64()
	if !ok || int64(int(n)) != n {
		return 0, fmt.Errorf("%s: %s is out of range", fn, name)
	}
	return int(n), nil
}

func int64Arg(fn, name string, value starlark.Int) (int64, error) {
	n, ok := value.Int64()
	if !ok {
		return 0, fmt.Errorf("%s: %s is out of range", fn, name)
	}
	return n, nil
}

func numberArg(fn, name string, value starlark.Value) (float64, error) {
	switch v := value.(type) {
	case starlark.Int:
		n, ok := v.Int64()
		if !ok {
			return 0, fmt.Errorf("%s: %s is out of range", fn, name)
		}
		return float64(n), nil
	case starlark.Float:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("%s: %s must be int or float", fn, name)
	}
}

func statValue(info fs.FileInfo) starlark.Value {
	mode := info.Mode()
	mtime := info.ModTime()
	return &statResultValue{
		mode:  int64(mode),
		size:  info.Size(),
		mtime: float64(mtime.UnixNano()) / 1e9,
		atime: float64(mtime.UnixNano()) / 1e9,
		ctime: float64(mtime.UnixNano()) / 1e9,
		ino:   0,
		dev:   0,
		isDir: info.IsDir(),
	}
}

type statResultValue struct {
	mode  int64
	size  int64
	mtime float64
	atime float64
	ctime float64
	ino   int64
	dev   int64
	isDir bool
}

func (s *statResultValue) String() string { return "os.stat_result" }

func (s *statResultValue) Type() string { return "os.stat_result" }

func (s *statResultValue) Freeze() {}

func (s *statResultValue) Truth() starlark.Bool { return starlark.True }

func (s *statResultValue) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: os.stat_result")
}

func (s *statResultValue) AttrNames() []string {
	return []string{"is_dir", "st_atime", "st_ctime", "st_dev", "st_ino", "st_mode", "st_mtime", "st_size"}
}

func (s *statResultValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "st_mode":
		return starlark.MakeInt64(s.mode), nil
	case "st_size":
		return starlark.MakeInt64(s.size), nil
	case "st_mtime":
		return starlark.Float(s.mtime), nil
	case "st_atime":
		return starlark.Float(s.atime), nil
	case "st_ctime":
		return starlark.Float(s.ctime), nil
	case "st_ino":
		return starlark.MakeInt64(s.ino), nil
	case "st_dev":
		return starlark.MakeInt64(s.dev), nil
	case "is_dir":
		return starlark.Bool(s.isDir), nil
	default:
		return nil, nil
	}
}

func permissionsAllow(mode fs.FileMode, req int) bool {
	if req == 0 {
		return true
	}
	perm := mode.Perm()
	if req&4 != 0 && perm&0o444 == 0 {
		return false
	}
	if req&2 != 0 && perm&0o222 == 0 {
		return false
	}
	if req&1 != 0 && perm&0o111 == 0 {
		return false
	}
	return true
}

// listdir implements os.listdir for string paths.

func (f FileSystem) listdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	path := "."
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path?", &path); err != nil {
		return nil, err
	}
	entries, err := f.fsys.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	items := make([]starlark.Value, len(entries))
	for i, entry := range entries {
		items[i] = starlark.String(entry.Name())
	}
	return starlark.NewList(items), nil
}

func (f FileSystem) scandir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	path := "."
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path?", &path); err != nil {
		return nil, err
	}
	entries, err := f.fsys.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	items := make([]starlark.Value, len(entries))
	for i, entry := range entries {
		items[i] = &dirEntryValue{fsys: f.fsys, stat: statResultFromDirEntry(entry), dir: path, name: entry.Name(), mode: entry.Type()}
	}
	return starlark.NewList(items), nil
}

func (f FileSystem) walk(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var top string
	topdown := true
	followlinks := false
	var onerror starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "top", &top, "topdown?", &topdown, "onerror?", &onerror, "followlinks?", &followlinks); err != nil {
		return nil, err
	}
	var out []starlark.Value
	if err := f.walkInto(top, topdown, followlinks, &out); err != nil {
		if onerror != starlark.None {
			_, callErr := starlark.Call(thread, onerror, starlark.Tuple{starlark.String(err.Error())}, nil)
			return starlark.NewList(out), callErr
		}
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	return starlark.NewList(out), nil
}

func (f FileSystem) walkInto(path string, topdown, followlinks bool, out *[]starlark.Value) error {
	entries, err := f.fsys.ReadDir(path)
	if err != nil {
		return err
	}
	var dirs, files []starlark.Value
	var childDirs []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			dirs = append(dirs, starlark.String(name))
			childDirs = append(childDirs, joinPath(path, name))
		} else {
			files = append(files, starlark.String(name))
		}
	}
	tuple := starlark.Tuple{starlark.String(path), starlark.NewList(dirs), starlark.NewList(files)}
	if topdown {
		*out = append(*out, tuple)
	}
	for _, child := range childDirs {
		if err := f.walkInto(child, topdown, followlinks, out); err != nil {
			return err
		}
	}
	if !topdown {
		*out = append(*out, tuple)
	}
	return nil
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

func (f FileSystem) stat(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	info, err := f.fsys.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	return statValue(info), nil
}

func (f FileSystem) lstat(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	info, err := f.fsys.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	return statValue(info), nil
}

func (f FileSystem) access(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	modeVal := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path, "mode", &modeVal); err != nil {
		return nil, err
	}
	mode, err := intArg(fn.Name(), "mode", modeVal)
	if err != nil {
		return nil, err
	}
	info, err := f.fsys.Stat(path)
	return starlark.Bool(err == nil && permissionsAllow(info.Mode(), mode)), nil
}

func (f FileSystem) mkdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	modeVal := starlark.MakeInt(0o777)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path, "mode?", &modeVal); err != nil {
		return nil, err
	}
	mode, err := intArg(fn.Name(), "mode", modeVal)
	if err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Mkdir(path, fs.FileMode(mode))
}

func (f FileSystem) makedirs(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	modeVal := starlark.MakeInt(0o777)
	existOK := false
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &path, "mode?", &modeVal, "exist_ok?", &existOK); err != nil {
		return nil, err
	}
	mode, err := intArg(fn.Name(), "mode", modeVal)
	if err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	clean := filepath.ToSlash(path)
	prefix := ""
	if strings.HasPrefix(clean, "/") {
		prefix = "/"
	}
	parts := strings.Split(strings.Trim(clean, "/"), "/")
	curr := prefix
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}
		curr = joinPath(curr, part)
		err := fsys.Mkdir(curr, fs.FileMode(mode))
		if err == nil {
			continue
		}
		if isExist(err) && (i < len(parts)-1 || existOK) {
			continue
		}
		return nil, err
	}
	return starlark.None, nil
}

func (f FileSystem) rmdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return f.remove(thread, fn, args, kwargs)
}

func (f FileSystem) removedirs(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &path); err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	first := true
	for path != "" && path != "." {
		if err := fsys.Remove(path); err != nil {
			if first {
				return nil, err
			}
			return starlark.None, nil
		}
		first = false
		path, _ = splitPath(path)
	}
	return starlark.None, nil
}

func (f FileSystem) remove(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Remove(path)
}

func (f FileSystem) rename(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dst string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dst", &dst); err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Rename(src, dst)
}

func (f FileSystem) renames(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var oldname, newname string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "old", &oldname, "new", &newname); err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	if _, tail := splitPath(newname); tail != "" {
		dir, _ := splitPath(newname)
		if dir != "" {
			if _, err := f.makedirs(thread, starlark.NewBuiltin(fn.Name(), f.makedirs), starlark.Tuple{starlark.String(dir)}, nil); err != nil {
				return nil, err
			}
		}
	}
	if err := fsys.Rename(oldname, newname); err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func splitPath(path string) (string, string) {
	path = strings.TrimRight(path, "/")
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "", path
	}
	return path[:i], path[i+1:]
}

func isExist(err error) bool { return errors.Is(err, fs.ErrExist) }

func (f FileSystem) chmod(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	modeVal := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path, "mode", &modeVal); err != nil {
		return nil, err
	}
	mode, err := intArg(fn.Name(), "mode", modeVal)
	if err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Chmod(path, fs.FileMode(mode))
}

func (f FileSystem) chown(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	uidVal, gidVal := starlark.MakeInt(0), starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path, "uid", &uidVal, "gid", &gidVal); err != nil {
		return nil, err
	}
	uid, err := intArg(fn.Name(), "uid", uidVal)
	if err != nil {
		return nil, err
	}
	gid, err := intArg(fn.Name(), "gid", gidVal)
	if err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Chown(path, uid, gid)
}

func (f FileSystem) utime(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var timesVal starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path, "times?", &timesVal); err != nil {
		return nil, err
	}
	atime, mtime := time.Now(), time.Now()
	if timesVal != starlark.None {
		seq, ok := timesVal.(starlark.Indexable)
		if !ok || seq.Len() != 2 {
			return nil, fmt.Errorf("%s: times must be a 2-item sequence", fn.Name())
		}
		a, err := numberArg(fn.Name(), "times[0]", seq.Index(0))
		if err != nil {
			return nil, err
		}
		m, err := numberArg(fn.Name(), "times[1]", seq.Index(1))
		if err != nil {
			return nil, err
		}
		atime, mtime = unixFloat(a), unixFloat(m)
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Chtimes(path, atime, mtime)
}

func unixFloat(seconds float64) time.Time {
	whole := int64(seconds)
	frac := seconds - float64(whole)
	return time.Unix(whole, int64(frac*1e9))
}

func (f FileSystem) truncate(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	sizeVal := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path, "length", &sizeVal); err != nil {
		return nil, err
	}
	size, err := int64Arg(fn.Name(), "length", sizeVal)
	if err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Truncate(path, size)
}

func (f FileSystem) link(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dst string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dst", &dst); err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Link(src, dst)
}

func (f FileSystem) symlink(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dst string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dst", &dst); err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Symlink(src, dst)
}

func (f FileSystem) readlink(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	fsys, err := f.mut(fn.Name())
	if err != nil {
		return nil, err
	}
	value, err := fsys.Readlink(path)
	if err != nil {
		return nil, err
	}
	return starlark.String(value), nil
}

func (f FileSystem) open(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	flagVal := starlark.MakeInt(f.platform.OpenFlags.ReadOnly)
	modeVal := starlark.MakeInt(0o777)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path, "flags", &flagVal, "mode?", &modeVal); err != nil {
		return nil, err
	}
	flag, err := intArg(fn.Name(), "flags", flagVal)
	if err != nil {
		return nil, err
	}
	mode, err := intArg(fn.Name(), "mode", modeVal)
	if err != nil {
		return nil, err
	}
	fsys, err := f.openfs(fn.Name())
	if err != nil {
		return nil, err
	}
	file, err := fsys.OpenFile(path, flag, fs.FileMode(mode))
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(f.fds.Store(file)), nil
}

func (f FileSystem) fileForFD(fn string, fdVal starlark.Int) (xfs.File, int, error) {
	fd, err := intArg(fn, "fd", fdVal)
	if err != nil {
		return nil, 0, err
	}
	file, ok := f.fds.Lookup(fd)
	if !ok {
		return nil, 0, fmt.Errorf("%s: unknown file descriptor %d", fn, fd)
	}
	return file, fd, nil
}

func (f FileSystem) close(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	fdVal := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "fd", &fdVal); err != nil {
		return nil, err
	}
	file, fd, err := f.fileForFD(fn.Name(), fdVal)
	if err != nil {
		return nil, err
	}
	f.fds.Delete(fd)
	return starlark.None, file.Close()
}

func (f FileSystem) read(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	fdVal, nVal := starlark.MakeInt(0), starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "fd", &fdVal, "n", &nVal); err != nil {
		return nil, err
	}
	file, _, err := f.fileForFD(fn.Name(), fdVal)
	if err != nil {
		return nil, err
	}
	n, err := intArg(fn.Name(), "n", nVal)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	read, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return starlark.Bytes(string(buf[:read])), nil
}

func (f FileSystem) write(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	fdVal := starlark.MakeInt(0)
	var data starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "fd", &fdVal, "data", &data); err != nil {
		return nil, err
	}
	file, _, err := f.fileForFD(fn.Name(), fdVal)
	if err != nil {
		return nil, err
	}
	var buf []byte
	switch v := data.(type) {
	case starlark.String:
		buf = []byte(string(v))
	case starlark.Bytes:
		buf = []byte(string(v))
	default:
		return nil, fmt.Errorf("%s: data must be str or bytes", fn.Name())
	}
	n, err := file.Write(buf)
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(n), nil
}

func (f FileSystem) fsync(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	fdVal := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "fd", &fdVal); err != nil {
		return nil, err
	}
	file, _, err := f.fileForFD(fn.Name(), fdVal)
	if err != nil {
		return nil, err
	}
	return starlark.None, file.Sync()
}

func (f FileSystem) ftruncate(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	fdVal, sizeVal := starlark.MakeInt(0), starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "fd", &fdVal, "length", &sizeVal); err != nil {
		return nil, err
	}
	file, _, err := f.fileForFD(fn.Name(), fdVal)
	if err != nil {
		return nil, err
	}
	size, err := int64Arg(fn.Name(), "length", sizeVal)
	if err != nil {
		return nil, err
	}
	return starlark.None, file.Truncate(size)
}

func (f FileSystem) pathAbspath(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	fsys, err := f.pathfs(fn.Name())
	if err != nil {
		return nil, err
	}
	value, err := fsys.Abs(path)
	if err != nil {
		return nil, err
	}
	return starlark.String(filepath.ToSlash(value)), nil
}

func (f FileSystem) pathExists(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	_, err := f.fsys.Stat(path)
	return starlark.Bool(err == nil), nil
}

func (f FileSystem) pathLexists(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	_, err := f.fsys.Lstat(path)
	return starlark.Bool(err == nil), nil
}

func (f FileSystem) pathGetatime(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return f.pathTime(fn, args, kwargs)
}

func (f FileSystem) pathGetmtime(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return f.pathTime(fn, args, kwargs)
}

func (f FileSystem) pathGetctime(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return f.pathTime(fn, args, kwargs)
}

func (f FileSystem) pathTime(fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "filename", &path); err != nil {
		return nil, err
	}
	info, err := f.fsys.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	return starlark.Float(float64(info.ModTime().UnixNano()) / 1e9), nil
}

func (f FileSystem) pathGetsize(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "filename", &path); err != nil {
		return nil, err
	}
	info, err := f.fsys.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	return starlark.MakeInt64(info.Size()), nil
}

func (f FileSystem) pathIsDir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	info, err := f.fsys.Stat(path)
	return starlark.Bool(err == nil && info.IsDir()), nil
}

func (f FileSystem) pathIsFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	info, err := f.fsys.Stat(path)
	return starlark.Bool(err == nil && info.Mode().IsRegular()), nil
}

func (f FileSystem) pathIslink(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	info, err := f.fsys.Lstat(path)
	return starlark.Bool(err == nil && info.Mode()&fs.ModeSymlink != 0), nil
}

func (f FileSystem) pathIsmount(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	clean := filepath.Clean(path)
	return starlark.Bool(clean == "/" || filepath.VolumeName(clean)+string(filepath.Separator) == clean), nil
}

func (f FileSystem) pathRealpath(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "filename", &path); err != nil {
		return nil, err
	}
	fsys, err := f.pathfs(fn.Name())
	if err != nil {
		return nil, err
	}
	value, err := fsys.Realpath(path)
	if err != nil {
		return nil, err
	}
	return starlark.String(filepath.ToSlash(value)), nil
}

func (f FileSystem) pathSamefile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var a, b string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "f1", &a, "f2", &b); err != nil {
		return nil, err
	}
	fsys, err := f.samefile(fn.Name())
	if err != nil {
		return nil, err
	}
	same, err := fsys.SameFile(a, b)
	if err != nil {
		return nil, err
	}
	return starlark.Bool(same), nil
}

type dirEntryValue struct {
	fsys xfs.FS
	stat *statResultValue
	dir  string
	name string
	mode fs.FileMode
}

func statResultFromDirEntry(entry fs.DirEntry) *statResultValue {
	if info, err := entry.Info(); err == nil {
		return statValue(info).(*statResultValue)
	}
	return &statResultValue{mode: int64(entry.Type()), isDir: entry.IsDir()}
}

func (d *dirEntryValue) String() string { return "<DirEntry " + d.name + ">" }

func (d *dirEntryValue) Type() string { return "os.DirEntry" }

func (d *dirEntryValue) Freeze() {}

func (d *dirEntryValue) Truth() starlark.Bool { return starlark.True }

func (d *dirEntryValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: os.DirEntry") }

func (d *dirEntryValue) AttrNames() []string {
	return []string{"is_dir", "is_file", "name", "path", "stat"}
}

func (d *dirEntryValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "name":
		return starlark.String(d.name), nil
	case "path":
		return starlark.String(joinPath(d.dir, d.name)), nil
	case "is_dir", "is_file", "stat":
		return dirEntryMethods[name].BindReceiver(d), nil
	default:
		return nil, nil
	}
}

func dirEntryIsDir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	d := fn.Receiver().(*dirEntryValue)
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.Bool(d.stat != nil && d.stat.isDir), nil
}

func dirEntryIsFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	d := fn.Receiver().(*dirEntryValue)
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.Bool(d.stat != nil && !d.stat.isDir && d.mode.IsRegular()), nil
}

func dirEntryStat(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	d := fn.Receiver().(*dirEntryValue)
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	if d.fsys != nil {
		info, err := d.fsys.Stat(joinPath(d.dir, d.name))
		if err != nil {
			return nil, err
		}
		return statValue(info), nil
	}
	if d.stat != nil {
		return d.stat, nil
	}
	return nil, fmt.Errorf("%s: stat snapshot is not available", fn.Name())
}
