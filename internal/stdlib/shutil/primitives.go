package shutil

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"path/filepath"
	"strings"

	"github.com/spachava753/dyson/internal/xfs"
	"github.com/spachava753/dyson/internal/xos"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

const defaultSearchPath = "/bin:/usr/bin"

type primitives struct {
	fsys     xfs.FS
	env      xos.Env
	terminal xos.Terminal
	platform xos.Platform
}

type moduleFunctions struct {
	os         *starlarkstruct.Module
	primitives primitives
}

func (p primitives) copyFile(fn, src, dst string) (string, error) {
	fsys, ok := p.fsys.(xfs.OpenFS)
	if !ok {
		return "", fmt.Errorf("%s: filesystem does not support file descriptors", fn)
	}
	source, err := fsys.OpenFile(src, p.platform.OpenFlags.ReadOnly, 0)
	if err != nil {
		return "", err
	}
	defer source.Close()
	destination, err := fsys.OpenFile(dst, p.platform.OpenFlags.WriteOnly|p.platform.OpenFlags.Create|p.platform.OpenFlags.Truncate, 0o666)
	if err != nil {
		return "", err
	}
	defer destination.Close()
	if _, err := io.Copy(destination, source); err != nil {
		return "", err
	}
	return filepath.ToSlash(dst), nil
}

func (p primitives) diskUsage(fn, path string) (starlark.Value, error) {
	fsys, ok := p.fsys.(xfs.UsageFS)
	if !ok {
		return nil, fmt.Errorf("%s: filesystem does not support disk usage", fn)
	}
	usage, err := fsys.DiskUsage(path)
	if err != nil {
		return nil, err
	}
	return starlark.Tuple{starlark.MakeUint64(usage.Total), starlark.MakeUint64(usage.Used), starlark.MakeUint64(usage.Free)}, nil
}

func (p primitives) getenv(key string, fallback starlark.Value) starlark.Value {
	if p.env != nil {
		if value, ok := p.env.LookupEnv(key); ok {
			return starlark.String(value)
		}
	}
	return fallback
}

func (m moduleFunctions) copyfile(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dst string
	followSymlinks := true
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dst", &dst, "follow_symlinks?", &followSymlinks); err != nil {
		return nil, err
	}
	return m.copyfileImpl(fn.Name(), src, dst, followSymlinks)
}

func (m moduleFunctions) copyfileImpl(fn, src, dst string, followSymlinks bool) (starlark.Value, error) {
	if !followSymlinks {
		return nil, fmt.Errorf("%s: follow_symlinks=False is not supported", fn)
	}
	if m.primitives.fsys != nil {
		if _, err := m.primitives.fsys.Stat(dst); err == nil {
			sameFile, ok := m.primitives.fsys.(xfs.SameFileFS)
			if !ok {
				return nil, fmt.Errorf("%s: filesystem cannot check whether src and dst are the same file", fn)
			}
			same, err := sameFile.SameFile(src, dst)
			if err != nil {
				return nil, err
			}
			if same {
				return nil, fmt.Errorf("%s: src and dst are the same file", fn)
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	copied, err := m.primitives.copyFile(fn, src, dst)
	if err != nil {
		return nil, err
	}
	return starlark.String(copied), nil
}

func (m moduleFunctions) copymode(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dst string
	followSymlinks := true
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dst", &dst, "follow_symlinks?", &followSymlinks); err != nil {
		return nil, err
	}
	return m.copyMetadata(thread, fn.Name(), src, dst, followSymlinks, false)
}

func (m moduleFunctions) copystat(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dst string
	followSymlinks := true
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dst", &dst, "follow_symlinks?", &followSymlinks); err != nil {
		return nil, err
	}
	return m.copyMetadata(thread, fn.Name(), src, dst, followSymlinks, true)
}

func (m moduleFunctions) copyMetadata(thread *starlark.Thread, fn, src, dst string, followSymlinks, times bool) (starlark.Value, error) {
	if !followSymlinks {
		return nil, fmt.Errorf("%s: follow_symlinks=False is not supported", fn)
	}
	stat, err := m.callOS(thread, "stat", starlark.String(src))
	if err != nil {
		return nil, err
	}
	mode, err := attribute(stat, "st_mode")
	if err != nil {
		return nil, err
	}
	if _, err := m.callOS(thread, "chmod", starlark.String(dst), mode); err != nil {
		return nil, err
	}
	if times {
		atime, err := attribute(stat, "st_atime")
		if err != nil {
			return nil, err
		}
		mtime, err := attribute(stat, "st_mtime")
		if err != nil {
			return nil, err
		}
		if _, err := m.callOS(thread, "utime", starlark.String(dst), starlark.Tuple{atime, mtime}); err != nil {
			return nil, err
		}
	}
	return starlark.None, nil
}

func (m moduleFunctions) copy(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return m.unpackCopy(thread, fn, args, kwargs, false)
}

func (m moduleFunctions) copy2(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return m.unpackCopy(thread, fn, args, kwargs, true)
}

func (m moduleFunctions) unpackCopy(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple, preserveTimes bool) (starlark.Value, error) {
	var src, dst string
	followSymlinks := true
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dst", &dst, "follow_symlinks?", &followSymlinks); err != nil {
		return nil, err
	}
	return m.copyImpl(thread, src, dst, followSymlinks, preserveTimes)
}

func (m moduleFunctions) copyImpl(thread *starlark.Thread, src, dst string, followSymlinks, preserveTimes bool) (starlark.Value, error) {
	isDir, err := m.pathTruth(thread, "isdir", dst)
	if err != nil {
		return nil, err
	}
	if isDir {
		basename, err := m.pathString(thread, "basename", src)
		if err != nil {
			return nil, err
		}
		dst, err = m.pathString(thread, "join", dst, basename)
		if err != nil {
			return nil, err
		}
	}
	if _, err := m.copyfileImpl(ModuleName+".copyfile", src, dst, followSymlinks); err != nil {
		return nil, err
	}
	metadataFunction := ModuleName + ".copymode"
	if preserveTimes {
		metadataFunction = ModuleName + ".copystat"
	}
	if _, err := m.copyMetadata(thread, metadataFunction, src, dst, followSymlinks, preserveTimes); err != nil {
		return nil, err
	}
	return starlark.String(dst), nil
}

func (m moduleFunctions) copytree(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dst string
	symlinks, ignoreDanglingSymlinks, dirsExistOK := false, false, false
	var ignore, copyFunction starlark.Value = starlark.None, starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"src", &src, "dst", &dst, "symlinks?", &symlinks, "ignore?", &ignore,
		"copy_function?", &copyFunction, "ignore_dangling_symlinks?", &ignoreDanglingSymlinks,
		"dirs_exist_ok?", &dirsExistOK,
	); err != nil {
		return nil, err
	}
	if symlinks {
		return nil, fmt.Errorf("%s: symlinks=True is not supported", fn.Name())
	}
	if ignoreDanglingSymlinks {
		return nil, fmt.Errorf("%s: ignore_dangling_symlinks is not supported", fn.Name())
	}
	return m.copytreeImpl(thread, fn.Name(), src, dst, ignore, copyFunction, dirsExistOK)
}

func (m moduleFunctions) copytreeImpl(thread *starlark.Thread, fn, src, dst string, ignore, copyFunction starlark.Value, dirsExistOK bool) (starlark.Value, error) {
	exists, err := m.pathTruth(thread, "exists", dst)
	if err != nil {
		return nil, err
	}
	if exists && !dirsExistOK {
		return nil, fmt.Errorf("%s: destination exists", fn)
	}
	if exists {
		directory, err := m.pathTruth(thread, "isdir", dst)
		if err != nil {
			return nil, err
		}
		if !directory {
			return nil, fmt.Errorf("%s: destination exists and is not a directory", fn)
		}
	}
	if !exists {
		if _, err := m.callOS(thread, "makedirs", starlark.String(dst)); err != nil {
			return nil, err
		}
	}
	names, err := m.callOS(thread, "listdir", starlark.String(src))
	if err != nil {
		return nil, err
	}
	ignored := starlark.Value(starlark.NewList(nil))
	if ignore != starlark.None {
		ignored, err = starlark.Call(thread, ignore, starlark.Tuple{starlark.String(src), names}, nil)
		if err != nil {
			return nil, err
		}
	}
	iterator := starlark.Iterate(names)
	if iterator == nil {
		return nil, fmt.Errorf("%s: os.listdir returned non-iterable %s", fn, names.Type())
	}
	defer iterator.Done()
	var nameValue starlark.Value
	for iterator.Next(&nameValue) {
		contained, err := starlark.Binary(syntax.IN, nameValue, ignored)
		if err != nil {
			return nil, err
		}
		if bool(contained.Truth()) {
			continue
		}
		name, ok := starlark.AsString(nameValue)
		if !ok {
			return nil, fmt.Errorf("%s: os.listdir item is not a string", fn)
		}
		sourcePath, err := m.pathString(thread, "join", src, name)
		if err != nil {
			return nil, err
		}
		destinationPath, err := m.pathString(thread, "join", dst, name)
		if err != nil {
			return nil, err
		}
		directory, err := m.pathTruth(thread, "isdir", sourcePath)
		if err != nil {
			return nil, err
		}
		if directory {
			if _, err := m.copytreeImpl(thread, fn, sourcePath, destinationPath, ignore, copyFunction, dirsExistOK); err != nil {
				return nil, err
			}
		} else if copyFunction != starlark.None {
			if _, err := starlark.Call(thread, copyFunction, starlark.Tuple{starlark.String(sourcePath), starlark.String(destinationPath)}, nil); err != nil {
				return nil, err
			}
		} else if _, err := m.copyImpl(thread, sourcePath, destinationPath, true, true); err != nil {
			return nil, err
		}
	}
	if _, err := m.copyMetadata(thread, ModuleName+".copystat", src, dst, true, true); err != nil {
		return nil, err
	}
	return starlark.String(dst), nil
}

var errRmtreeSymlink = errors.New("cannot call rmtree on a symbolic link")

func (m moduleFunctions) rmtree(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	ignoreErrors := false
	var onerror, onexc, dirFD starlark.Value = starlark.None, starlark.None, starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"path", &path, "ignore_errors?", &ignoreErrors, "onerror?", &onerror,
		"onexc?", &onexc, "dir_fd?", &dirFD,
	); err != nil {
		return nil, err
	}
	if onerror != starlark.None {
		return nil, fmt.Errorf("%s: onerror is not supported", fn.Name())
	}
	if onexc != starlark.None {
		return nil, fmt.Errorf("%s: onexc is not supported", fn.Name())
	}
	if dirFD != starlark.None {
		return nil, fmt.Errorf("%s: dir_fd is not supported", fn.Name())
	}
	err := m.primitives.removeTree(path)
	if err == nil || ignoreErrors {
		return starlark.None, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: path does not exist", fn.Name())
	}
	if errors.Is(err, errRmtreeSymlink) {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	return nil, err
}

func (p primitives) removeTree(path string) error {
	if p.fsys == nil {
		return fmt.Errorf("shutil.rmtree: filesystem is not configured")
	}
	remover, ok := p.fsys.(xfs.RemoveTreeFS)
	if !ok {
		return fmt.Errorf("shutil.rmtree: filesystem does not support safe recursive deletion")
	}
	info, err := p.fsys.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return errRmtreeSymlink
	}
	if !info.IsDir() {
		return fmt.Errorf("shutil.rmtree: path is not a directory")
	}
	return remover.RemoveTree(path)
}

func (m moduleFunctions) move(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dst string
	var copyFunction starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dst", &dst, "copy_function?", &copyFunction); err != nil {
		return nil, err
	}
	_ = copyFunction
	realDestination := dst
	isDir, err := m.pathTruth(thread, "isdir", dst)
	if err != nil {
		return nil, err
	}
	if isDir {
		basename, err := m.pathString(thread, "basename", src)
		if err != nil {
			return nil, err
		}
		realDestination, err = m.pathString(thread, "join", dst, basename)
		if err != nil {
			return nil, err
		}
	}
	exists, err := m.pathTruth(thread, "exists", realDestination)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%s: destination exists", fn.Name())
	}
	if _, err := m.callOS(thread, "rename", starlark.String(src), starlark.String(realDestination)); err != nil {
		return nil, err
	}
	return starlark.String(realDestination), nil
}

func (m moduleFunctions) diskUsage(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	return m.primitives.diskUsage(fn.Name(), path)
}

func (m moduleFunctions) chown(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var user, group, dirFD starlark.Value = starlark.None, starlark.None, starlark.None
	followSymlinks := true
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"path", &path, "user?", &user, "group?", &group, "dir_fd?", &dirFD, "follow_symlinks?", &followSymlinks,
	); err != nil {
		return nil, err
	}
	if dirFD != starlark.None {
		return nil, fmt.Errorf("%s: dir_fd is not supported", fn.Name())
	}
	if !followSymlinks {
		return nil, fmt.Errorf("%s: follow_symlinks=False is not supported", fn.Name())
	}
	if user == starlark.None {
		user = starlark.MakeInt(-1)
	}
	if group == starlark.None {
		group = starlark.MakeInt(-1)
	}
	if _, ok := user.(starlark.Int); !ok {
		return nil, fmt.Errorf("%s: user and group must be int or None", fn.Name())
	}
	if _, ok := group.(starlark.Int); !ok {
		return nil, fmt.Errorf("%s: user and group must be int or None", fn.Name())
	}
	if _, err := m.callOS(thread, "chown", starlark.String(path), user, group); err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func (m moduleFunctions) getTerminalSize(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fallback starlark.Value = starlark.Tuple{starlark.MakeInt(80), starlark.MakeInt(24)}
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "fallback?", &fallback); err != nil {
		return nil, err
	}
	columns, columnsOK := positiveInt(m.primitives.getenv("COLUMNS", starlark.None))
	lines, linesOK := positiveInt(m.primitives.getenv("LINES", starlark.None))
	if columnsOK && linesOK {
		return starlark.Tuple{columns, lines}, nil
	}
	if m.primitives.terminal == nil {
		return fallback, nil
	}
	columnsValue, linesValue, err := m.primitives.terminal.TerminalSize()
	if err != nil {
		return fallback, nil
	}
	return starlark.Tuple{starlark.MakeInt(columnsValue), starlark.MakeInt(linesValue)}, nil
}

func (m moduleFunctions) which(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var command string
	var mode, searchPath starlark.Value = starlark.None, starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &command, "mode?", &mode, "path?", &searchPath); err != nil {
		return nil, err
	}
	if mode == starlark.None {
		fOK, err := attribute(m.os, "F_OK")
		if err != nil {
			return nil, err
		}
		xOK, err := attribute(m.os, "X_OK")
		if err != nil {
			return nil, err
		}
		mode, err = starlark.Binary(syntax.PIPE, fOK, xOK)
		if err != nil {
			return nil, err
		}
	}
	if containsSlash(command) {
		return m.executable(thread, command, mode)
	}
	if searchPath == starlark.None {
		searchPath = m.primitives.getenv("PATH", starlark.String(defaultSearchPath))
	}
	pathText, ok := starlark.AsString(searchPath)
	if !ok {
		return nil, fmt.Errorf("%s: path must be a string", fn.Name())
	}
	separator := m.primitives.platform.PathListSeparator
	if separator == "" {
		separator = ":"
	}
	for directory := range strings.SplitSeq(pathText, separator) {
		if directory == "" {
			directory = "."
		}
		candidate, err := m.pathString(thread, "join", filepath.ToSlash(directory), command)
		if err != nil {
			return nil, err
		}
		result, err := m.executable(thread, candidate, mode)
		if err != nil {
			return nil, err
		}
		if result != starlark.None {
			return result, nil
		}
	}
	return starlark.None, nil
}

func (m moduleFunctions) executable(thread *starlark.Thread, candidate string, mode starlark.Value) (starlark.Value, error) {
	accessible, err := m.osTruth(thread, "access", starlark.String(candidate), mode)
	if err != nil || !accessible {
		return starlark.None, err
	}
	directory, err := m.pathTruth(thread, "isdir", candidate)
	if err != nil || directory {
		return starlark.None, err
	}
	return starlark.String(candidate), nil
}

func (m moduleFunctions) callOS(thread *starlark.Thread, name string, args ...starlark.Value) (starlark.Value, error) {
	return callAttribute(thread, m.os, name, starlark.Tuple(args))
}

func (m moduleFunctions) callPath(thread *starlark.Thread, name string, args ...starlark.Value) (starlark.Value, error) {
	path, err := attribute(m.os, "path")
	if err != nil {
		return nil, err
	}
	return callAttribute(thread, path, name, starlark.Tuple(args))
}

func (m moduleFunctions) osTruth(thread *starlark.Thread, name string, args ...starlark.Value) (bool, error) {
	value, err := m.callOS(thread, name, args...)
	if err != nil {
		return false, err
	}
	return bool(value.Truth()), nil
}

func (m moduleFunctions) pathTruth(thread *starlark.Thread, name string, args ...string) (bool, error) {
	values := make([]starlark.Value, len(args))
	for i, arg := range args {
		values[i] = starlark.String(arg)
	}
	value, err := m.callPath(thread, name, values...)
	if err != nil {
		return false, err
	}
	return bool(value.Truth()), nil
}

func (m moduleFunctions) pathString(thread *starlark.Thread, name string, args ...string) (string, error) {
	values := make([]starlark.Value, len(args))
	for i, arg := range args {
		values[i] = starlark.String(arg)
	}
	value, err := m.callPath(thread, name, values...)
	if err != nil {
		return "", err
	}
	result, ok := starlark.AsString(value)
	if !ok {
		return "", fmt.Errorf("os.path.%s returned %s, want string", name, value.Type())
	}
	return result, nil
}

func callAttribute(thread *starlark.Thread, receiver starlark.Value, name string, args starlark.Tuple) (starlark.Value, error) {
	callable, err := attribute(receiver, name)
	if err != nil {
		return nil, err
	}
	return starlark.Call(thread, callable, args, nil)
}

func attribute(receiver starlark.Value, name string) (starlark.Value, error) {
	hasAttrs, ok := receiver.(starlark.HasAttrs)
	if !ok {
		return nil, fmt.Errorf("%s has no attributes", receiver.Type())
	}
	value, err := hasAttrs.Attr(name)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("%s has no attribute %q", receiver.Type(), name)
	}
	return value, nil
}

func positiveInt(value starlark.Value) (starlark.Int, bool) {
	if integer, ok := value.(starlark.Int); ok {
		return integer, integer.Sign() > 0
	}
	text, ok := starlark.AsString(value)
	if !ok || text == "" {
		return starlark.Int{}, false
	}
	integer, ok := new(big.Int).SetString(text, 10)
	if !ok || integer.Sign() <= 0 {
		return starlark.Int{}, false
	}
	return starlark.MakeBigInt(integer), true
}

func containsSlash(path string) bool { return strings.ContainsAny(path, `/\`) }
