# Starlark implementation of Dyson's Python-like glob compatibility module.
#
# The filesystem operations intentionally go through os and os.path so the glob
# logic can become useful as those lower-level modules are implemented.

def _glob_public(pathname, root_dir=None, dir_fd=None, recursive=False, include_hidden=False):
    """Return a list of paths matching pathname."""
    return _iglob_public(pathname, root_dir=root_dir, dir_fd=dir_fd, recursive=recursive, include_hidden=include_hidden)


def _iglob_public(pathname, root_dir=None, dir_fd=None, recursive=False, include_hidden=False):
    """Return paths matching pathname.

    Python returns an iterator. Dyson currently returns a list because Go
    Starlark does not provide Python generator semantics.
    """
    if root_dir != None:
        fail("glob.iglob: root_dir is not supported")
    if dir_fd != None:
        fail("glob.iglob: dir_fd is not supported")

    if not _has_magic(pathname):
        if _lexists(pathname):
            return [pathname]
        return []

    prefix, segments = _split_segments(pathname)
    return _glob_segments(prefix, segments, recursive, include_hidden)


def _escape_public(pathname):
    """Escape glob metacharacters in pathname."""
    result = ""
    for i in range(len(pathname)):
        c = pathname[i]
        if c in ("*", "?", "["):
            result += "[" + c + "]"
        else:
            result += c
    return result


def _glob_segments(prefix, segments, recursive, include_hidden):
    if len(segments) == 0:
        if prefix != "" and _lexists(prefix):
            return [prefix]
        return []

    segment = segments[0]
    rest = segments[1:]

    if segment == "**" and recursive:
        if len(rest) == 0:
            return _walk_all(prefix, include_hidden)

        results = []
        _append_all(results, _glob_segments(prefix, rest, recursive, include_hidden))
        for dirname in _walk_dirs(prefix, include_hidden):
            _append_all(results, _glob_segments(dirname, rest, recursive, include_hidden))
        return results

    if not _has_magic(segment):
        path = _join(prefix, segment)
        if len(rest) == 0:
            if _lexists(path):
                return [path]
            return []
        if _isdir(path):
            return _glob_segments(path, rest, recursive, include_hidden)
        return []

    results = []
    for name in _listdir(prefix):
        if not include_hidden and not _is_hidden(segment) and _is_hidden(name):
            continue
        if not _fnmatchcase(name, segment):
            continue

        path = _join(prefix, name)
        if len(rest) == 0:
            results.append(path)
        elif _isdir(path):
            _append_all(results, _glob_segments(path, rest, recursive, include_hidden))
    return results


def _walk_all(prefix, include_hidden):
    results = []
    for name in _listdir(prefix):
        if not include_hidden and _is_hidden(name):
            continue
        path = _join(prefix, name)
        results.append(path)
        if _isdir(path):
            _append_all(results, _walk_all(path, include_hidden))
    return results


def _walk_dirs(prefix, include_hidden):
    results = []
    for name in _listdir(prefix):
        if not include_hidden and _is_hidden(name):
            continue
        path = _join(prefix, name)
        if _isdir(path):
            results.append(path)
            _append_all(results, _walk_dirs(path, include_hidden))
    return results


def _append_all(dst, src):
    for item in src:
        dst.append(item)


def _listdir(dirname):
    if dirname == "":
        dirname = os.curdir
    return os.listdir(dirname)


def _lexists(path):
    return os.path.lexists(path)


def _isdir(path):
    return os.path.isdir(path)


def _split_segments(pathname):
    sep = _sep()
    prefix = ""
    rest = pathname
    while rest.startswith(sep):
        prefix = sep
        rest = rest[len(sep):]

    segments = []
    for segment in rest.split(sep):
        if segment != "":
            segments.append(segment)
    return prefix, segments


def _join(dirname, basename):
    if dirname == "":
        return basename
    sep = _sep()
    if dirname.endswith(sep):
        return dirname + basename
    return dirname + sep + basename


def _sep():
    return os.sep


def _has_magic(pathname):
    for i in range(len(pathname)):
        if pathname[i] in ("*", "?", "["):
            return True
    return False


def _is_hidden(path):
    return path.startswith(".")


def _fnmatchcase(name, pattern):
    if not _has_magic(pattern):
        return name == pattern
    return re.fullmatch(_translate(pattern), name) != None


def _translate(pattern):
    i = 0
    result = ""
    while i < len(pattern):
        c = pattern[i]
        i += 1
        if c == "*":
            result += ".*"
        elif c == "?":
            result += "."
        elif c == "[":
            j = i
            if j < len(pattern) and pattern[j] in ("!", "]"):
                j += 1
            while j < len(pattern) and pattern[j] != "]":
                j += 1

            if j >= len(pattern):
                result += "\\["
            else:
                stuff = pattern[i:j]
                if stuff.startswith("!"):
                    stuff = "^" + stuff[1:]
                elif stuff.startswith("^"):
                    stuff = "\\" + stuff
                result += "[" + stuff + "]"
                i = j + 1
        else:
            result += re.escape(c)
    return result


# Python docs: https://docs.python.org/3/library/glob.html
glob = module("glob", glob=_glob_public, iglob=_iglob_public, escape=_escape_public)
