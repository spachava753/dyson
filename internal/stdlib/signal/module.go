package signal

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// ModuleName is the Starlark stdlib module name for Dyson's signal compatibility module.
const ModuleName = "signal"

const (
	signalsTypeName  = "signal.Signals"
	handlersTypeName = "signal.Handlers"
)

var knownSignals = []signalInfo{
	{name: "SIGHUP", number: 1, description: "Hangup"},
	{name: "SIGINT", number: 2, description: "Interrupt"},
	{name: "SIGQUIT", number: 3, description: "Quit"},
	{name: "SIGILL", number: 4, description: "Illegal instruction"},
	{name: "SIGTRAP", number: 5, description: "Trace/breakpoint trap"},
	{name: "SIGABRT", number: 6, description: "Aborted"},
	{name: "SIGBUS", number: 7, description: "Bus error"},
	{name: "SIGFPE", number: 8, description: "Floating point exception"},
	{name: "SIGKILL", number: 9, description: "Killed"},
	{name: "SIGUSR1", number: 10, description: "User defined signal 1"},
	{name: "SIGSEGV", number: 11, description: "Segmentation fault"},
	{name: "SIGUSR2", number: 12, description: "User defined signal 2"},
	{name: "SIGPIPE", number: 13, description: "Broken pipe"},
	{name: "SIGALRM", number: 14, description: "Alarm clock"},
	{name: "SIGTERM", number: 15, description: "Terminated"},
}

var signalsByNumber = func() map[int]signalInfo {
	out := make(map[int]signalInfo, len(knownSignals))
	for _, sig := range knownSignals {
		out[sig.number] = sig
	}
	return out
}()

var signalValuesByNumber = func() map[int]*enumValue {
	out := make(map[int]*enumValue, len(knownSignals))
	for _, sig := range knownSignals {
		out[sig.number] = &enumValue{typ: signalsTypeName, info: enumInfo{name: sig.name, number: sig.number}}
	}
	return out
}()

var handlersByNumber = map[int]enumInfo{
	0: {name: "SIG_DFL", number: 0},
	1: {name: "SIG_IGN", number: 1},
}

var handlerValuesByNumber = map[int]*enumValue{
	0: {typ: handlersTypeName, info: handlersByNumber[0]},
	1: {typ: handlersTypeName, info: handlersByNumber[1]},
}

// Module is the Starlark module namespace exposed by load("signal.star", "signal").
var Module = func() *starlarkstruct.Module {
	members := starlark.StringDict{
		"strsignal":     starlark.NewBuiltin(ModuleName+".strsignal", strsignal),
		"valid_signals": starlark.NewBuiltin(ModuleName+".valid_signals", validSignals),
		"Signals":       starlark.NewBuiltin(ModuleName+".Signals", signals),
		"Handlers":      starlark.NewBuiltin(ModuleName+".Handlers", handlers),
		"SIG_DFL":       handlerValuesByNumber[0],
		"SIG_IGN":       handlerValuesByNumber[1],
	}
	for _, sig := range knownSignals {
		members[sig.name] = signalValuesByNumber[sig.number]
	}
	members["SIGIOT"] = members["SIGABRT"]
	m := &starlarkstruct.Module{Name: ModuleName, Members: members}
	m.Freeze()
	return m
}()

type signalInfo struct {
	name        string
	number      int
	description string
}

type enumInfo struct {
	name   string
	number int
}

func strsignal(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	number, err := signalNumber(fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}
	if sig, ok := signalsByNumber[number]; ok {
		return starlark.String(sig.description), nil
	}
	return nil, fmt.Errorf("%s: signal number out of range", fn.Name())
}

func validSignals(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	set := starlark.NewSet(len(knownSignals))
	for _, sig := range knownSignals {
		if err := set.Insert(signalValuesByNumber[sig.number]); err != nil {
			return nil, err
		}
	}
	return set, nil
}

func signals(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	number, err := signalNumber(fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}
	sig, ok := signalsByNumber[number]
	if !ok {
		return nil, fmt.Errorf("%s: %d is not a valid Signals", fn.Name(), number)
	}
	return signalValuesByNumber[sig.number], nil
}

func handlers(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	number, err := signalNumber(fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}
	if _, ok := handlersByNumber[number]; !ok {
		return nil, fmt.Errorf("%s: %d is not a valid Handlers", fn.Name(), number)
	}
	return handlerValuesByNumber[number], nil
}

func signalNumber(fn string, args starlark.Tuple, kwargs []starlark.Tuple) (int, error) {
	var val starlark.Value
	if err := starlark.UnpackArgs(fn, args, kwargs, "signalnum", &val); err != nil {
		return 0, err
	}
	return intValue(fn, "signalnum", val)
}

func intValue(fn, name string, val starlark.Value) (int, error) {
	switch v := val.(type) {
	case starlark.Int:
		number, ok := v.Int64()
		if !ok || int64(int(number)) != number {
			return 0, fmt.Errorf("%s: %s is out of range", fn, name)
		}
		return int(number), nil
	case *enumValue:
		return v.info.number, nil
	default:
		return 0, fmt.Errorf("%s: %s must be an int", fn, name)
	}
}

type enumValue struct {
	typ  string
	info enumInfo
}

func (e *enumValue) String() string {
	return strings.TrimPrefix(e.typ, ModuleName+".") + "." + e.info.name
}
func (e *enumValue) Type() string { return e.typ }
func (e *enumValue) Freeze()      {}
func (e *enumValue) Truth() starlark.Bool {
	return starlark.Bool(e.info.number != 0)
}
func (e *enumValue) Hash() (uint32, error) {
	return starlark.MakeInt(e.info.number).Hash()
}
func (e *enumValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "name":
		return starlark.String(e.info.name), nil
	case "value":
		return starlark.MakeInt(e.info.number), nil
	}
	return nil, nil
}
func (e *enumValue) AttrNames() []string {
	names := []string{"name", "value"}
	sort.Strings(names)
	return names
}
func (e *enumValue) Binary(op syntax.Token, y starlark.Value, side starlark.Side) (starlark.Value, error) {
	switch op {
	case syntax.EQL:
		return starlark.Bool(enumEqual(e, y)), nil
	case syntax.NEQ:
		return starlark.Bool(!enumEqual(e, y)), nil
	}
	return nil, nil
}

func enumEqual(e *enumValue, other starlark.Value) bool {
	switch v := other.(type) {
	case *enumValue:
		return e.info.number == v.info.number
	case starlark.Int:
		number, ok := v.Int64()
		return ok && number == int64(e.info.number)
	default:
		return false
	}
}

func (e *enumValue) GoString() string {
	return "<" + strings.TrimPrefix(e.typ, ModuleName+".") + "." + e.info.name + ": " + strconv.Itoa(e.info.number) + ">"
}
