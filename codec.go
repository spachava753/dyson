package dyson

import (
	"github.com/spachava753/dyson/internal/codec"
	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	stdlibre "github.com/spachava753/dyson/internal/stdlib/re"
	stdlibrequests "github.com/spachava753/dyson/internal/stdlib/requests"
	stdlibshutil "github.com/spachava753/dyson/internal/stdlib/shutil"
	stdlibsignal "github.com/spachava753/dyson/internal/stdlib/signal"
	stdlibsubprocess "github.com/spachava753/dyson/internal/stdlib/subprocess"
	stdlibtime "github.com/spachava753/dyson/internal/stdlib/time"
)

// DefaultCodecRegistry returns a fresh registry containing Dyson's built-in
// durable host-call codecs, including codecs for supported stdlib custom
// values.
func DefaultCodecRegistry() codec.Registry {
	registry := codec.DefaultRegistry()
	stdlibos.RegisterCodecs(registry)
	stdlibre.RegisterCodecs(registry)
	stdlibrequests.RegisterCodecs(registry)
	stdlibshutil.RegisterCodecs(registry)
	stdlibsignal.RegisterCodecs(registry)
	stdlibsubprocess.RegisterCodecs(registry)
	stdlibtime.RegisterStructTimeCodec(registry)
	return registry
}
