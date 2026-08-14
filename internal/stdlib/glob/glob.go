package glob

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/starlarkstruct"
)

type moduleImplementation struct {
	os *starlarkstruct.Module
}

func (g moduleImplementation) glob(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pathname string
	var rootDir, dirFD starlark.Value = starlark.None, starlark.None
	recursive, includeHidden := false, false
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"pathname", &pathname, "root_dir?", &rootDir, "dir_fd?", &dirFD,
		"recursive?", &recursive, "include_hidden?", &includeHidden,
	); err != nil {
		return nil, err
	}
	if rootDir != starlark.None {
		return nil, fmt.Errorf("glob.iglob: root_dir is not supported")
	}
	if dirFD != starlark.None {
		return nil, fmt.Errorf("glob.iglob: dir_fd is not supported")
	}
	if !hasMagic(pathname) {
		exists, err := g.pathTest(thread, "lexists", pathname)
		if err != nil || !exists {
			return stringList(nil), err
		}
		return stringList([]string{pathname}), nil
	}
	prefix, segments := splitSegments(pathname)
	results, err := g.globSegments(thread, prefix, segments, recursive, includeHidden)
	if err != nil {
		return nil, err
	}
	return stringList(results), nil
}

// globSegments recursively resolves pathname segments beneath prefix. It handles
// recursive ** as zero or more directories, validates literal segments, and
// applies hidden-file policy while matching wildcard segments.
func (g moduleImplementation) globSegments(thread *starlark.Thread, prefix string, segments []string, recursive, includeHidden bool) ([]string, error) {
	if len(segments) == 0 {
		if prefix == "" {
			return nil, nil
		}
		exists, err := g.pathTest(thread, "lexists", prefix)
		if err != nil || !exists {
			return nil, err
		}
		return []string{prefix}, nil
	}

	segment, rest := segments[0], segments[1:]
	if segment == "**" && recursive {
		if len(rest) == 0 {
			return g.walk(thread, prefix, includeHidden, true)
		}
		results, err := g.globSegments(thread, prefix, rest, recursive, includeHidden)
		if err != nil {
			return nil, err
		}
		directories, err := g.walk(thread, prefix, includeHidden, false)
		if err != nil {
			return nil, err
		}
		for _, directory := range directories {
			matches, err := g.globSegments(thread, directory, rest, recursive, includeHidden)
			if err != nil {
				return nil, err
			}
			results = append(results, matches...)
		}
		return results, nil
	}

	if !hasMagic(segment) {
		candidate := join(prefix, segment)
		if len(rest) == 0 {
			exists, err := g.pathTest(thread, "lexists", candidate)
			if err != nil || !exists {
				return nil, err
			}
			return []string{candidate}, nil
		}
		directory, err := g.pathTest(thread, "isdir", candidate)
		if err != nil || !directory {
			return nil, err
		}
		return g.globSegments(thread, candidate, rest, recursive, includeHidden)
	}

	names, err := g.listdir(thread, prefix)
	if err != nil {
		return nil, err
	}
	matcher, err := compilePattern(segment)
	if err != nil {
		return nil, err
	}
	var results []string
	for _, name := range names {
		if !includeHidden && !isHidden(segment) && isHidden(name) {
			continue
		}
		if !matcher.MatchString(name) {
			continue
		}
		candidate := join(prefix, name)
		if len(rest) == 0 {
			results = append(results, candidate)
			continue
		}
		directory, err := g.pathTest(thread, "isdir", candidate)
		if err != nil {
			return nil, err
		}
		if directory {
			matches, err := g.globSegments(thread, candidate, rest, recursive, includeHidden)
			if err != nil {
				return nil, err
			}
			results = append(results, matches...)
		}
	}
	return results, nil
}

// walk recursively lists descendants beneath prefix for ** expansion. It can
// return files and directories or directories only, filters hidden names when
// requested, and never descends through symbolic links.
func (g moduleImplementation) walk(thread *starlark.Thread, prefix string, includeHidden, includeFiles bool) ([]string, error) {
	names, err := g.listdir(thread, prefix)
	if err != nil {
		return nil, err
	}
	var results []string
	for _, name := range names {
		if !includeHidden && isHidden(name) {
			continue
		}
		candidate := join(prefix, name)
		if includeFiles {
			results = append(results, candidate)
		}
		directory, err := g.pathTest(thread, "isdir", candidate)
		if err != nil {
			return nil, err
		}
		if !directory {
			continue
		}
		if !includeFiles {
			results = append(results, candidate)
		}
		link, err := g.pathTest(thread, "islink", candidate)
		if err != nil {
			return nil, err
		}
		if link {
			continue
		}
		nested, err := g.walk(thread, candidate, includeHidden, includeFiles)
		if err != nil {
			return nil, err
		}
		results = append(results, nested...)
	}
	return results, nil
}

func (g moduleImplementation) listdir(thread *starlark.Thread, directory string) ([]string, error) {
	if directory == "" {
		directory = "."
	}
	value, err := g.callOS(thread, "listdir", starlark.String(lookupPath(directory)))
	if err != nil {
		return nil, err
	}
	list, ok := value.(*starlark.List)
	if !ok {
		return nil, fmt.Errorf("glob: os.listdir returned %s, want list", value.Type())
	}
	result := make([]string, list.Len())
	for i := range list.Len() {
		name, ok := starlark.AsString(list.Index(i))
		if !ok {
			return nil, fmt.Errorf("glob: os.listdir item %d is not a string", i)
		}
		result[i] = name
	}
	return result, nil
}

func (g moduleImplementation) pathTest(thread *starlark.Thread, name, pathname string) (bool, error) {
	if g.os == nil {
		return false, fmt.Errorf("glob: os module is not configured")
	}
	pathValue, err := g.os.Attr("path")
	if err != nil {
		return false, err
	}
	pathModule, ok := pathValue.(*starlarkstruct.Module)
	if !ok {
		return false, fmt.Errorf("glob: os.path is not a module")
	}
	function, err := pathModule.Attr(name)
	if err != nil {
		return false, err
	}
	value, err := starlark.Call(thread, function, starlark.Tuple{starlark.String(lookupPath(pathname))}, nil)
	if err != nil {
		return false, err
	}
	return bool(value.Truth()), nil
}

func (g moduleImplementation) callOS(thread *starlark.Thread, name string, args ...starlark.Value) (starlark.Value, error) {
	if g.os == nil {
		return nil, fmt.Errorf("glob: os module is not configured")
	}
	function, err := g.os.Attr(name)
	if err != nil {
		return nil, err
	}
	return starlark.Call(thread, function, starlark.Tuple(args), nil)
}

func lookupPath(pathname string) string {
	for strings.HasPrefix(pathname, "./") {
		pathname = strings.TrimLeft(pathname[2:], "/")
	}
	if pathname == "" {
		return "."
	}
	return pathname
}

func escape(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pathname string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "pathname", &pathname); err != nil {
		return nil, err
	}
	var result strings.Builder
	for _, character := range pathname {
		if strings.ContainsRune("*?[", character) {
			result.WriteRune('[')
			result.WriteRune(character)
			result.WriteRune(']')
		} else {
			result.WriteRune(character)
		}
	}
	return starlark.String(result.String()), nil
}

func splitSegments(pathname string) (string, []string) {
	prefix, rest := "", pathname
	for strings.HasPrefix(rest, "/") {
		prefix, rest = "/", strings.TrimPrefix(rest, "/")
	}
	var segments []string
	for segment := range strings.SplitSeq(rest, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return prefix, segments
}

func join(directory, basename string) string {
	if directory == "" {
		return basename
	}
	if strings.HasSuffix(directory, "/") {
		return directory + basename
	}
	return directory + "/" + basename
}

func hasMagic(pathname string) bool { return strings.ContainsAny(pathname, "*?[") }
func isHidden(pathname string) bool { return strings.HasPrefix(pathname, ".") }

// compilePattern translates one Python-style glob segment into an anchored
// regular expression. It supports *, ?, character classes, class negation, and
// treats an unmatched opening bracket literally.
func compilePattern(pattern string) (*regexp.Regexp, error) {
	var expression strings.Builder
	expression.WriteByte('^')
	for i := 0; i < len(pattern); {
		character := pattern[i]
		i++
		switch character {
		case '*':
			expression.WriteString(".*")
		case '?':
			expression.WriteByte('.')
		case '[':
			end := i
			if end < len(pattern) && (pattern[end] == '!' || pattern[end] == ']') {
				end++
			}
			for end < len(pattern) && pattern[end] != ']' {
				end++
			}
			if end == len(pattern) {
				expression.WriteString(`\[`)
				continue
			}
			class := pattern[i:end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			} else if strings.HasPrefix(class, "^") {
				class = `\` + class
			}
			expression.WriteByte('[')
			expression.WriteString(class)
			expression.WriteByte(']')
			i = end + 1
		default:
			expression.WriteString(regexp.QuoteMeta(string(character)))
		}
	}
	expression.WriteByte('$')
	compiled, err := regexp.Compile(expression.String())
	if err != nil {
		return nil, fmt.Errorf("glob: invalid pattern %q: %w", pattern, err)
	}
	return compiled, nil
}

func stringList(values []string) *starlark.List {
	items := make([]starlark.Value, len(values))
	for i, value := range values {
		items[i] = starlark.String(value)
	}
	return starlark.NewList(items)
}
