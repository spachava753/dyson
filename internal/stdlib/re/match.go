package re

import (
	"fmt"

	"go.starlark.net/starlark"
)

func (m *matchValue) String() string {
	return fmt.Sprintf("<re.Match object; span=(%d, %d), match=%s>", m.index[0], m.index[1], m.input.starlarkValue(m.groupString(0)).String())
}
func (m *matchValue) Type() string          { return "re.Match" }
func (m *matchValue) Freeze()               { m.frozen = true }
func (m *matchValue) Truth() starlark.Bool  { return starlark.True }
func (m *matchValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: re.Match") }

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
		return starlark.NewBuiltin("re.Match."+name, m.method(name)), nil
	}
	return nil, nil
}

func (m *matchValue) AttrNames() []string { return matchAttrNames }

func (m *matchValue) method(name string) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		switch name {
		case "expand":
			var templateValue starlark.Value
			if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "template", &templateValue); err != nil {
				return nil, err
			}
			template, err := regexTextFromValue(fn.Name(), "template", templateValue)
			if err != nil {
				return nil, err
			}
			return template.starlarkValue(expandReplacement(template.text, m)), nil
		case "group":
			if len(args) == 0 && len(kwargs) == 0 {
				return m.group(0)
			}
			if len(kwargs) > 0 {
				return nil, fmt.Errorf("%s: unexpected keyword arguments", fn.Name())
			}
			values := make(starlark.Tuple, len(args))
			for i, arg := range args {
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
		case "groups":
			var defaultValue starlark.Value = starlark.None
			if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "default?", &defaultValue); err != nil {
				return nil, err
			}
			items := make(starlark.Tuple, m.pattern.groups)
			for group := 1; group <= m.pattern.groups; group++ {
				value, _ := m.group(group)
				if value == starlark.None {
					value = defaultValue
				}
				items[group-1] = value
			}
			return items, nil
		case "groupdict":
			var defaultValue starlark.Value = starlark.None
			if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "default?", &defaultValue); err != nil {
				return nil, err
			}
			dict := starlark.NewDict(m.pattern.groupIndex.Len())
			for i, name := range m.pattern.groupNames {
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
		case "start", "end", "span":
			group, err := unpackOptionalGroup(fn.Name(), args, kwargs)
			if err != nil {
				return nil, err
			}
			start, end, err := m.spanForValue(group)
			if err != nil {
				return nil, err
			}
			switch name {
			case "start":
				return starlark.MakeInt(start), nil
			case "end":
				return starlark.MakeInt(end), nil
			default:
				return starlark.Tuple{starlark.MakeInt(start), starlark.MakeInt(end)}, nil
			}
		}
		return nil, nil
	}
}

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

func (m *matchValue) lastIndex() starlark.Value {
	last := m.lastIndexValue()
	if last == 0 {
		return starlark.None
	}
	return starlark.MakeInt(last)
}

func (m *matchValue) lastIndexValue() int {
	for group := m.pattern.groups; group >= 1; group-- {
		if m.index[group*2] >= 0 {
			return group
		}
	}
	return 0
}

func unpackOptionalGroup(fn string, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var group starlark.Value = starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn, args, kwargs, "group?", &group); err != nil {
		return nil, err
	}
	return group, nil
}

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
