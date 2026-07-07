package signal

import (
	"fmt"

	"github.com/spachava753/dyson/internal/codec"
	"go.starlark.net/starlark"
)

// RegisterCodecs installs durable serialization support for signal.Signals
// values into a codec registry.
func RegisterCodecs(registry codec.Registry) {
	registry.Register(codec.ValueCodec{
		Type:    signalsTypeName,
		Version: 1,
		Serialize: func(val starlark.Value) (codec.SerializedVal, error) {
			sig, ok := val.(*enumValue)
			if !ok || sig.typ != signalsTypeName {
				return codec.SerializedVal{}, fmt.Errorf("dyson: got %T for signal.Signals codec", val)
			}
			number, err := registry.Serialize(starlark.MakeInt(sig.info.number))
			if err != nil {
				return codec.SerializedVal{}, err
			}
			return codec.SerializedVal{Type: signalsTypeName, List: []codec.SerializedVal{number}}, nil
		},
		Restore: func(val codec.SerializedVal) (starlark.Value, error) {
			if len(val.List) != 1 {
				return nil, fmt.Errorf("dyson: invalid signal.Signals payload length %d", len(val.List))
			}
			numberValue, err := registry.Restore(val.List[0])
			if err != nil {
				return nil, err
			}
			numberInt, ok := numberValue.(starlark.Int)
			if !ok {
				return nil, fmt.Errorf("dyson: signal.Signals value must restore to int, got %s", numberValue.Type())
			}
			number, ok := numberInt.Int64()
			if !ok || int64(int(number)) != number {
				return nil, fmt.Errorf("dyson: signal.Signals value is out of range")
			}
			info, ok := signalsByNumber[int(number)]
			if !ok {
				return nil, fmt.Errorf("dyson: signal.Signals value %d is not valid", number)
			}
			return signalValuesByNumber[info.number], nil
		},
	})
}
