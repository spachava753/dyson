package dyson

import (
	"github.com/spachava753/dyson/internal/codec"
	stdlibtime "github.com/spachava753/dyson/internal/stdlib/time"
)

// DefaultCodecRegistry returns a fresh registry containing Dyson's built-in
// durable host-call codecs, including codecs for supported stdlib custom
// values.
func DefaultCodecRegistry() codec.Registry {
	registry := codec.DefaultRegistry()
	stdlibtime.RegisterStructTimeCodec(registry)
	return registry
}
