package subprocess

import (
	"fmt"

	"github.com/spachava753/dyson/internal/codec"
	"go.starlark.net/starlark"
)

// RegisterCodecs installs durable serialization support for
// subprocess.CompletedProcess values into a codec registry.
func RegisterCodecs(registry codec.Registry) {
	registry.Register(codec.ValueCodec{
		Type:    completedProcessTypeName,
		Version: 1,
		Serialize: func(val starlark.Value) (codec.SerializedVal, error) {
			p, ok := val.(*completedProcessValue)
			if !ok {
				return codec.SerializedVal{}, fmt.Errorf("dyson: got %T for subprocess.CompletedProcess codec", val)
			}
			args, err := registry.Serialize(p.args)
			if err != nil {
				return codec.SerializedVal{}, err
			}
			returncode, err := registry.Serialize(starlark.MakeInt(p.returncode))
			if err != nil {
				return codec.SerializedVal{}, err
			}
			stdout, err := registry.Serialize(p.stdout)
			if err != nil {
				return codec.SerializedVal{}, err
			}
			stderr, err := registry.Serialize(p.stderr)
			if err != nil {
				return codec.SerializedVal{}, err
			}
			return codec.SerializedVal{Type: completedProcessTypeName, List: []codec.SerializedVal{args, returncode, stdout, stderr}}, nil
		},
		Restore: func(val codec.SerializedVal) (starlark.Value, error) {
			if len(val.List) != 4 {
				return nil, fmt.Errorf("dyson: invalid subprocess.CompletedProcess payload length %d", len(val.List))
			}
			args, err := registry.Restore(val.List[0])
			if err != nil {
				return nil, err
			}
			returncodeValue, err := registry.Restore(val.List[1])
			if err != nil {
				return nil, err
			}
			returncodeInt, ok := returncodeValue.(starlark.Int)
			if !ok {
				return nil, fmt.Errorf("dyson: subprocess.CompletedProcess returncode must restore to int, got %s", returncodeValue.Type())
			}
			returncode, ok := returncodeInt.Int64()
			if !ok || int64(int(returncode)) != returncode {
				return nil, fmt.Errorf("dyson: subprocess.CompletedProcess returncode is out of range")
			}
			stdout, err := registry.Restore(val.List[2])
			if err != nil {
				return nil, err
			}
			stderr, err := registry.Restore(val.List[3])
			if err != nil {
				return nil, err
			}
			return &completedProcessValue{args: args, returncode: int(returncode), stdout: stdout, stderr: stderr}, nil
		},
	})
}
