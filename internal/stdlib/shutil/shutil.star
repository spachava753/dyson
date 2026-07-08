# Starlark implementation of Dyson's Python-like shutil compatibility module.
#
# File-tree orchestration lives here. Host-facing operations go through _shutil
# primitives and the injected os module so callers can choose or deny filesystem,
# environment, terminal, and platform capabilities.


def _copyfile(src, dst, follow_symlinks=True):
    if follow_symlinks != True:
        fail("shutil.copyfile: follow_symlinks=False is not supported")
    if os.path.exists(dst) and os.path.samefile(src, dst):
        fail("shutil.copyfile: src and dst are the same file")
    return _shutil.copyfile(src, dst)


def _copymode(src, dst, follow_symlinks=True):
    if follow_symlinks != True:
        fail("shutil.copymode: follow_symlinks=False is not supported")
    os.chmod(dst, os.stat(src).st_mode)
    return None


def _copystat(src, dst, follow_symlinks=True):
    if follow_symlinks != True:
        fail("shutil.copystat: follow_symlinks=False is not supported")
    st = os.stat(src)
    os.chmod(dst, st.st_mode)
    os.utime(dst, (st.st_atime, st.st_mtime))
    return None


def _copy(src, dst, follow_symlinks=True):
    if os.path.isdir(dst):
        dst = os.path.join(dst, os.path.basename(src))
    _copyfile(src, dst, follow_symlinks=follow_symlinks)
    _copymode(src, dst, follow_symlinks=follow_symlinks)
    return dst


def _copy2(src, dst, follow_symlinks=True):
    if os.path.isdir(dst):
        dst = os.path.join(dst, os.path.basename(src))
    _copyfile(src, dst, follow_symlinks=follow_symlinks)
    _copystat(src, dst, follow_symlinks=follow_symlinks)
    return dst


def _ignore_patterns(*patterns):
    def _ignore(path, names):
        ignored = []
        for name in names:
            for pattern in patterns:
                if _fnmatchcase(name, pattern):
                    ignored.append(name)
                    break
        return ignored
    return _ignore


def _copytree(src, dst, symlinks=False, ignore=None, copy_function=None, ignore_dangling_symlinks=False, dirs_exist_ok=False):
    if symlinks:
        fail("shutil.copytree: symlinks=True is not supported")
    if ignore_dangling_symlinks:
        fail("shutil.copytree: ignore_dangling_symlinks is not supported")
    if copy_function == None:
        copy_function = _copy2
    if os.path.exists(dst):
        if not dirs_exist_ok:
            fail("shutil.copytree: destination exists")
    else:
        os.makedirs(dst)

    names = os.listdir(src)
    ignored = [] if ignore == None else ignore(src, names)
    for name in names:
        if name in ignored:
            continue
        s = os.path.join(src, name)
        d = os.path.join(dst, name)
        if os.path.isdir(s):
            _copytree(s, d, symlinks=symlinks, ignore=ignore, copy_function=copy_function, ignore_dangling_symlinks=ignore_dangling_symlinks, dirs_exist_ok=dirs_exist_ok)
        else:
            copy_function(s, d)
    _copystat(src, dst)
    return dst


def _rmtree(path, ignore_errors=False, onerror=None, onexc=None, dir_fd=None):
    if dir_fd != None:
        fail("shutil.rmtree: dir_fd is not supported")
    if onexc != None:
        fail("shutil.rmtree: onexc is not supported")
    if onerror != None:
        fail("shutil.rmtree: onerror is not supported")
    if not os.path.exists(path):
        if ignore_errors:
            return None
        fail("shutil.rmtree: path does not exist")
    for name in os.listdir(path):
        child = os.path.join(path, name)
        if os.path.isdir(child):
            _rmtree(child, ignore_errors=ignore_errors)
        else:
            os.unlink(child)
    os.rmdir(path)
    return None


def _move(src, dst, copy_function=None):
    if copy_function == None:
        copy_function = _copy2
    real_dst = dst
    if os.path.isdir(dst):
        real_dst = os.path.join(dst, os.path.basename(src))
    if os.path.exists(real_dst):
        fail("shutil.move: destination exists")
    try_rename = True
    if try_rename:
        os.rename(src, real_dst)
        return real_dst


def _disk_usage(path):
    return _shutil.disk_usage(path)


def _chown(path, user=None, group=None, dir_fd=None, follow_symlinks=True):
    if dir_fd != None:
        fail("shutil.chown: dir_fd is not supported")
    if follow_symlinks != True:
        fail("shutil.chown: follow_symlinks=False is not supported")
    if user == None:
        user = -1
    if group == None:
        group = -1
    if type(user) != "int" or type(group) != "int":
        fail("shutil.chown: user and group must be int or None")
    os.chown(path, user, group)
    return None


def _get_terminal_size(fallback=(80, 24)):
    columns = _to_positive_int(_shutil.getenv("COLUMNS", None))
    lines = _to_positive_int(_shutil.getenv("LINES", None))
    if columns != None and lines != None:
        return (columns, lines)
    size = _shutil.get_terminal_size()
    if size == None:
        return fallback
    return size


def _which(cmd, mode=None, path=None):
    if mode == None:
        mode = os.F_OK | os.X_OK
    if _has_dir(cmd):
        if os.access(cmd, mode) and not os.path.isdir(cmd):
            return cmd
        return None
    if path == None:
        path = _shutil.getenv("PATH", default_path)
    for dirname in _shutil.split_path(path):
        candidate = os.path.join(dirname, cmd)
        if os.access(candidate, mode) and not os.path.isdir(candidate):
            return candidate
    return None


def _to_positive_int(value):
    if value == None:
        return None
    if type(value) == "int":
        return value if value > 0 else None
    n = 0
    i = 0
    while i < len(value):
        ch = value[i]
        if ch < "0" or ch > "9":
            return None
        n = n * 10 + _digit(ch)
        i += 1
    return n if n > 0 else None


def _digit(ch):
    if ch == "0":
        return 0
    if ch == "1":
        return 1
    if ch == "2":
        return 2
    if ch == "3":
        return 3
    if ch == "4":
        return 4
    if ch == "5":
        return 5
    if ch == "6":
        return 6
    if ch == "7":
        return 7
    if ch == "8":
        return 8
    return 9


def _has_dir(path):
    return "/" in path or "\\" in path


def _fnmatchcase(name, pattern):
    if pattern == "*":
        return True
    if "*" not in pattern and "?" not in pattern:
        return name == pattern
    return _match_glob(name, pattern, 0, 0)


def _match_glob(name, pattern, i, j):
    while j < len(pattern):
        p = pattern[j]
        if p == "*":
            if j + 1 == len(pattern):
                return True
            k = i
            while k <= len(name):
                if _match_glob(name, pattern, k, j + 1):
                    return True
                k += 1
            return False
        if i >= len(name):
            return False
        if p != "?" and p != name[i]:
            return False
        i += 1
        j += 1
    return i == len(name)


# Python docs: https://docs.python.org/3/library/shutil.html
shutil = module(
    "shutil",
    copyfile=_copyfile,
    copymode=_copymode,
    copystat=_copystat,
    copy=_copy,
    copy2=_copy2,
    copytree=_copytree,
    rmtree=_rmtree,
    move=_move,
    disk_usage=_disk_usage,
    chown=_chown,
    get_terminal_size=_get_terminal_size,
    which=_which,
    ignore_patterns=_ignore_patterns,
)
