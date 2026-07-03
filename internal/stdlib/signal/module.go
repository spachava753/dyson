package signal

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's signal compatibility module.
const ModuleName = "signal"

// Module is the Starlark module namespace exposed by load("signal.star", "signal").
var Module = &starlarkstruct.Module{
	Name: ModuleName,
	Members: starlark.StringDict{
		// Python docs: https://docs.python.org/3/library/signal.html#signal.strsignal
		"strsignal": nil,
		// Python docs: https://docs.python.org/3/library/signal.html#signal.valid_signals
		"valid_signals": nil,
		// Python docs: https://docs.python.org/3/library/signal.html#signal.Signals
		"Signals": nil,
		// Python docs: https://docs.python.org/3/library/signal.html#signal.SIGINT
		"SIGINT": starlark.MakeInt(2),
		// Python docs: https://docs.python.org/3/library/signal.html#signal.SIGTERM
		"SIGTERM": starlark.MakeInt(15),
		// Python docs: https://docs.python.org/3/library/signal.html#signal.SIGKILL
		"SIGKILL": starlark.MakeInt(9),
		// Python docs: https://docs.python.org/3/library/signal.html#signal.SIGQUIT
		"SIGQUIT": starlark.MakeInt(3),
		// Python docs: https://docs.python.org/3/library/signal.html#signal.SIGHUP
		"SIGHUP": starlark.MakeInt(1),
	},
}
