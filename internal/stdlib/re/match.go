package re

import (
	"fmt"

	"github.com/spachava753/dyson/internal/xctx"
	"go.starlark.net/starlark"
)

// String returns a Python-like representation of the match object, including the
// matched span and group 0 text.
func (m *matchValue) String() string {
	return fmt.Sprintf("<re.Match object; span=(%d, %d), match=%s>", m.index[0], m.index[1], m.input.starlarkValue(m.groupString(0)).String())
}

// Type reports the Starlark-visible type name for regex matches.
func (m *matchValue) Type() string { return "re.Match" }

// Freeze marks the match immutable for Starlark's shared-value semantics.
func (m *matchValue) Freeze() { m.frozen = true }

// Truth reports that concrete match objects are always truthy.
func (m *matchValue) Truth() starlark.Bool { return starlark.True }

// Hash rejects hashing because Python re.Match objects are not hashable in this
// compatibility layer.
func (m *matchValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: re.Match") }

// Attr exposes Python-compatible Match attributes and bound methods.
//
// It mirrors the supported subset of Python's re.Match API:
// https://docs.python.org/3/library/re.html#match-objects
func (m *matchValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "pos":
		return starlark.MakeInt(m.pos), nil
	case "endpos":
		return starlark.MakeInt(m.endpos), nil
	case "lastindex":
		return m.lastIndex(), nil
	case "lastgroup":
		last := m.lastIndexValue()
		if last <= 0 || last >= len(m.pattern.groupNames) || m.pattern.groupNames[last] == "" {
			return starlark.None, nil
		}
		return starlark.String(m.pattern.groupNames[last]), nil
	case "re":
		return m.pattern, nil
	case "string":
		return m.input.value, nil
	case "expand", "group", "groups", "groupdict", "start", "end", "span":
		return matchMethods[name].BindReceiver(m), nil
	}
	return nil, nil
}

// AttrNames returns the names discoverable on Match values.
func (m *matchValue) AttrNames() []string { return matchAttrNames }

// matchExpand implements Match.expand.
func matchExpand(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	m := fn.Receiver().(*matchValue)
	var templateValue starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "template", &templateValue); err != nil {
		return nil, err
	}
	template, err := regexTextFromValue(fn.Name(), "template", templateValue)
	if err != nil {
		return nil, err
	}
	if err := xctx.Check(thread); err != nil {
		return nil, err
	}
	return template.starlarkValue(expandReplacement(template.text, m)), nil
}

// matchGroup implements Match.group.
func matchGroup(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	m := fn.Receiver().(*matchValue)
	if len(args) == 0 && len(kwargs) == 0 {
		return m.group(0)
	}
	if len(kwargs) > 0 {
		return nil, fmt.Errorf("%s: unexpected keyword arguments", fn.Name())
	}
	values := make(starlark.Tuple, len(args))
	for i, arg := range args {
		if err := xctx.Check(thread); err != nil {
			return nil, err
		}
		group, err := m.groupByValue(arg)
		if err != nil {
			return nil, err
		}
		values[i] = group
	}
	if len(values) == 1 {
		return values[0], nil
	}
	return values, nil
}

// matchGroups implements Match.groups.
func matchGroups(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	m := fn.Receiver().(*matchValue)
	var defaultValue starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "default?", &defaultValue); err != nil {
		return nil, err
	}
	items := make(starlark.Tuple, m.pattern.groups)
	for group := 1; group <= m.pattern.groups; group++ {
		if err := xctx.Check(thread); err != nil {
			return nil, err
		}
		value, _ := m.group(group)
		if value == starlark.None {
			value = defaultValue
		}
		items[group-1] = value
	}
	return items, nil
}

// matchGroupdict implements Match.groupdict.
func matchGroupdict(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	m := fn.Receiver().(*matchValue)
	var defaultValue starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "default?", &defaultValue); err != nil {
		return nil, err
	}
	dict := starlark.NewDict(m.pattern.groupIndex.Len())
	for i, name := range m.pattern.groupNames {
		if err := xctx.Check(thread); err != nil {
			return nil, err
		}
		if i == 0 || name == "" {
			continue
		}
		value, _ := m.group(i)
		if value == starlark.None {
			value = defaultValue
		}
		mustSet(dict, name, value)
	}
	return dict, nil
}

// matchStart implements Match.start.
func matchStart(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	start, _, err := matchSpan(thread, fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(start), nil
}

// matchEnd implements Match.end.
func matchEnd(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	_, end, err := matchSpan(thread, fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(end), nil
}

// matchSpanMethod implements Match.span.
func matchSpanMethod(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	start, end, err := matchSpan(thread, fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	return starlark.Tuple{starlark.MakeInt(start), starlark.MakeInt(end)}, nil
}

func matchSpan(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (int, int, error) {
	m := fn.Receiver().(*matchValue)
	group, err := unpackOptionalGroup(fn.Name(), args, kwargs)
	if err != nil {
		return 0, 0, err
	}
	if err := xctx.Check(thread); err != nil {
		return 0, 0, err
	}
	return m.spanForValue(group)
}

// groupByValue implements Match.group lookup for an integer or named group
// reference supplied from Starlark.
//
// It mirrors the supported subset of Python's Match.group:
// https://docs.python.org/3/library/re.html#re.Match.group
func (m *matchValue) groupByValue(value starlark.Value) (starlark.Value, error) {
	group, ok, err := groupNumber(value, m.pattern)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("no such group")
	}
	return m.group(group)
}

// group returns the captured text for a numeric group, None for an unmatched
// existing group, or an error for an out-of-range group.
func (m *matchValue) group(group int) (starlark.Value, error) {
	if group < 0 || group > m.pattern.groups {
		return nil, fmt.Errorf("no such group")
	}
	start, end := m.index[group*2], m.index[group*2+1]
	if start < 0 {
		return starlark.None, nil
	}
	return m.input.starlarkValue(m.input.text[start-m.input.offset : end-m.input.offset]), nil
}

// groupString returns the captured text for replacement expansion, using an
// empty string for invalid or unmatched groups as Python substitutions do.
func (m *matchValue) groupString(group int) string {
	if group < 0 || group > m.pattern.groups {
		return ""
	}
	start, end := m.index[group*2], m.index[group*2+1]
	if start < 0 {
		return ""
	}
	return m.input.text[start-m.input.offset : end-m.input.offset]
}

// spanForValue resolves an integer or named group reference and returns the
// group's start and end offsets.
//
// It mirrors the supported subset of Python's Match.span:
// https://docs.python.org/3/library/re.html#re.Match.span
func (m *matchValue) spanForValue(value starlark.Value) (int, int, error) {
	group, ok, err := groupNumber(value, m.pattern)
	if err != nil {
		return 0, 0, err
	}
	if !ok {
		return 0, 0, fmt.Errorf("no such group")
	}
	return m.index[group*2], m.index[group*2+1], nil
}

// lastIndex returns the Starlark value for Match.lastindex: the highest matched
// capturing group number, or None when no capturing group matched.
//
// It mirrors the supported subset of Python's Match.lastindex:
// https://docs.python.org/3/library/re.html#re.Match.lastindex
func (m *matchValue) lastIndex() starlark.Value {
	last := m.lastIndexValue()
	if last == 0 {
		return starlark.None
	}
	return starlark.MakeInt(last)
}

// lastIndexValue returns the numeric highest matched capturing group, or zero
// when no capturing group matched.
func (m *matchValue) lastIndexValue() int {
	for group := m.pattern.groups; group >= 1; group-- {
		if m.index[group*2] >= 0 {
			return group
		}
	}
	return 0
}

// unpackOptionalGroup decodes methods whose only optional parameter is a group
// reference, defaulting to group 0.
func unpackOptionalGroup(fn string, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var group starlark.Value = starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn, args, kwargs, "group?", &group); err != nil {
		return nil, err
	}
	return group, nil
}

// groupNumber resolves a Starlark integer or string group reference to a numeric
// group index and reports whether it exists on the pattern.
func groupNumber(value starlark.Value, pattern *patternValue) (int, bool, error) {
	switch v := value.(type) {
	case starlark.Int:
		n, ok := v.Int64()
		if !ok || int64(int(n)) != n {
			return 0, false, fmt.Errorf("group index out of range")
		}
		group := int(n)
		return group, group >= 0 && group <= pattern.groups, nil
	case starlark.String:
		index, found, err := pattern.groupIndex.Get(v)
		if err != nil || !found {
			return 0, false, err
		}
		group, err := starlark.AsInt32(index)
		return group, err == nil, nil
	default:
		return 0, false, fmt.Errorf("group must be int or str, got %s", value.Type())
	}
}
