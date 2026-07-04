# Starlark implementation of Dyson's Python-like os compatibility module.
#
# Deterministic path-string helpers live here. Host-facing operations are exposed
# through the injected _os primitive module and are intentionally stubbed in Go
# until their policy and behavior are implemented.


def _path_isabs(path):
    return path.startswith("/")


def _path_join(path, *parts):
    for part in parts:
        if part.startswith("/"):
            path = part
        elif path == "" or path.endswith("/"):
            path += part
        else:
            path += "/" + part
    return path


def _path_split(path):
    i = path.rfind("/") + 1
    head = path[:i]
    tail = path[i:]
    if head != "" and head != "/" * len(head):
        head = head.rstrip("/")
    return head, tail


def _path_splitdrive(path):
    return "", path


def _path_basename(path):
    return _path_split(path)[1]


def _path_dirname(path):
    return _path_split(path)[0]


def _path_splitext(path):
    sep = path.rfind("/")
    dot = path.rfind(".")
    if dot <= sep + 1:
        return path, ""
    return path[:dot], path[dot:]


def _path_normpath(path):
    if path == "":
        return "."

    initial_slashes = path.startswith("/")
    comps = []
    for comp in path.split("/"):
        if comp == "" or comp == ".":
            continue
        if comp != ".." or (not initial_slashes and len(comps) == 0) or (len(comps) > 0 and comps[-1] == ".."):
            comps.append(comp)
        elif len(comps) > 0:
            comps.pop()

    result = "/".join(comps)
    if initial_slashes:
        result = "/" + result
    if result == "":
        return "/" if initial_slashes else "."
    return result


def _path_relpath(path, start="."):
    path = _path_normpath(path)
    start = _path_normpath(start)
    if _path_isabs(path) != _path_isabs(start):
        fail("os.path.relpath: path and start must both be absolute or both be relative")

    path_parts = [] if path == "." else path.strip("/").split("/")
    start_parts = [] if start == "." else start.strip("/").split("/")
    i = 0
    while i < len(path_parts) and i < len(start_parts) and path_parts[i] == start_parts[i]:
        i += 1

    rel_parts = []
    for _ in start_parts[i:]:
        rel_parts.append("..")
    for part in path_parts[i:]:
        rel_parts.append(part)
    if len(rel_parts) == 0:
        return "."
    return "/".join(rel_parts)


def _path_commonpath(paths):
    if len(paths) == 0:
        fail("os.path.commonpath: arg is an empty sequence")

    first_abs = _path_isabs(paths[0])
    split_paths = []
    for path in paths:
        if _path_isabs(path) != first_abs:
            fail("os.path.commonpath: can't mix absolute and relative paths")
        parts = [] if path == "." else _path_normpath(path).strip("/").split("/")
        split_paths.append(parts)

    common = []
    i = 0
    while True:
        if i >= len(split_paths[0]):
            break
        part = split_paths[0][i]
        for parts in split_paths[1:]:
            if i >= len(parts) or parts[i] != part:
                return ("/" if first_abs else "") + "/".join(common)
        common.append(part)
        i += 1
    return ("/" if first_abs else "") + "/".join(common)


path = module(
    "os.path",
    abspath=_os.path_abspath,
    basename=_path_basename,
    dirname=_path_dirname,
    exists=_os.path_exists,
    lexists=_os.path_lexists,
    expanduser=_os.path_expanduser,
    expandvars=_os.path_expandvars,
    getatime=_os.path_getatime,
    getmtime=_os.path_getmtime,
    getctime=_os.path_getctime,
    getsize=_os.path_getsize,
    isabs=_path_isabs,
    isdir=_os.path_isdir,
    isfile=_os.path_isfile,
    islink=_os.path_islink,
    ismount=_os.path_ismount,
    join=_path_join,
    normpath=_path_normpath,
    realpath=_os.path_realpath,
    relpath=_path_relpath,
    samefile=_os.path_samefile,
    split=_path_split,
    splitdrive=_path_splitdrive,
    splitext=_path_splitext,
    commonpath=_path_commonpath,
    supports_unicode_filenames=True,
)

os = module(
    "os",
    getcwd=_os.getcwd,
    chdir=_os.chdir,
    get_exec_path=_os.get_exec_path,
    getpid=_os.getpid,
    getppid=_os.getppid,
    kill=_os.kill,
    system=_os.system,
    environ=_os.environ,
    getenv=_os.getenv,
    putenv=_os.putenv,
    unsetenv=_os.unsetenv,
    listdir=_os.listdir,
    scandir=_os.scandir,
    walk=_os.walk,
    stat=_os.stat,
    lstat=_os.lstat,
    access=_os.access,
    mkdir=_os.mkdir,
    makedirs=_os.makedirs,
    rmdir=_os.rmdir,
    removedirs=_os.removedirs,
    remove=_os.remove,
    unlink=_os.unlink,
    rename=_os.rename,
    replace=_os.replace,
    renames=_os.renames,
    chmod=_os.chmod,
    chown=_os.chown,
    utime=_os.utime,
    truncate=_os.truncate,
    link=_os.link,
    symlink=_os.symlink,
    readlink=_os.readlink,
    open=_os.open,
    close=_os.close,
    read=_os.read,
    write=_os.write,
    fsync=_os.fsync,
    ftruncate=_os.ftruncate,
    getuid=_os.getuid,
    geteuid=_os.geteuid,
    getgid=_os.getgid,
    getegid=_os.getegid,
    getgroups=_os.getgroups,
    umask=_os.umask,
    path=path,
    name=os_name,
    curdir=".",
    pardir="..",
    sep="/",
    altsep=None,
    extsep=".",
    pathsep=pathsep,
    linesep="\n",
    defpath="/bin:/usr/bin",
    devnull=devnull,
    F_OK=0,
    R_OK=4,
    W_OK=2,
    X_OK=1,
    O_RDONLY=o_rdonly,
    O_WRONLY=o_wronly,
    O_RDWR=o_rdwr,
    O_APPEND=o_append,
    O_CREAT=o_creat,
    O_EXCL=o_excl,
    O_SYNC=o_sync,
    O_TRUNC=o_trunc,
    SEEK_SET=0,
    SEEK_CUR=1,
    SEEK_END=2,
)
