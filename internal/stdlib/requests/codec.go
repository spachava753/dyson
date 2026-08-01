package requests

import (
	"fmt"

	"github.com/spachava753/dyson/internal/codec"
	"go.starlark.net/starlark"
)

// RegisterCodecs adds durable support for Response values. Redirect history is
// stored as flat snapshots so prefix histories do not recursively duplicate.
func RegisterCodecs(registry codec.Registry) {
	registry.Register(codec.ValueCodec{
		Type:    responseTypeName,
		Version: 1,
		Serialize: func(value starlark.Value) (codec.SerializedVal, error) {
			response, ok := value.(*responseValue)
			if !ok {
				return codec.SerializedVal{}, fmt.Errorf("dyson: got %T for %s codec", value, responseTypeName)
			}
			snapshots := make([]codec.SerializedVal, response.history.Len()+1)
			var err error
			snapshots[0], err = registry.Serialize(responseSnapshot(response))
			if err != nil {
				return codec.SerializedVal{}, err
			}
			for i := range response.history.Len() {
				previous, ok := response.history.Index(i).(*responseValue)
				if !ok {
					return codec.SerializedVal{}, fmt.Errorf("dyson: %s history item %d is %s", responseTypeName, i, response.history.Index(i).Type())
				}
				snapshots[i+1], err = registry.Serialize(responseSnapshot(previous))
				if err != nil {
					return codec.SerializedVal{}, err
				}
			}
			return codec.SerializedVal{List: snapshots}, nil
		},
		Restore: func(value codec.SerializedVal) (starlark.Value, error) {
			if len(value.List) == 0 {
				return nil, fmt.Errorf("dyson: invalid %s payload", responseTypeName)
			}
			mainSnapshot, err := registry.Restore(value.List[0])
			if err != nil {
				return nil, err
			}
			response, err := restoreResponseSnapshot(mainSnapshot)
			if err != nil {
				return nil, err
			}
			history := make([]starlark.Value, len(value.List)-1)
			for i, serialized := range value.List[1:] {
				restored, err := registry.Restore(serialized)
				if err != nil {
					return nil, err
				}
				previous, err := restoreResponseSnapshot(restored)
				if err != nil {
					return nil, err
				}
				previous.history = starlark.NewList(append([]starlark.Value(nil), history[:i]...))
				history[i] = previous
			}
			response.history = starlark.NewList(history)
			return response, nil
		},
	})
}

func responseSnapshot(response *responseValue) starlark.Tuple {
	return starlark.Tuple{
		starlark.MakeInt(response.statusCode),
		response.headers,
		response.content,
		starlark.String(response.url),
		starlark.String(response.reason),
		response.encoding,
	}
}

func restoreResponseSnapshot(value starlark.Value) (*responseValue, error) {
	items, ok := value.(starlark.Tuple)
	if !ok || len(items) != 6 {
		return nil, fmt.Errorf("dyson: %s snapshot must be a 6-item tuple", responseTypeName)
	}
	status, ok := items[0].(starlark.Int)
	if !ok {
		return nil, fmt.Errorf("dyson: %s status must be an int", responseTypeName)
	}
	statusCode, ok := status.Int64()
	if !ok {
		return nil, fmt.Errorf("dyson: %s status is out of range", responseTypeName)
	}
	headers, ok := items[1].(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("dyson: %s headers must be a dict", responseTypeName)
	}
	content, ok := items[2].(starlark.Bytes)
	if !ok {
		return nil, fmt.Errorf("dyson: %s content must be bytes", responseTypeName)
	}
	url, ok := starlark.AsString(items[3])
	if !ok {
		return nil, fmt.Errorf("dyson: %s URL must be a string", responseTypeName)
	}
	reason, ok := starlark.AsString(items[4])
	if !ok {
		return nil, fmt.Errorf("dyson: %s reason must be a string", responseTypeName)
	}
	if items[5] != starlark.None {
		if _, ok := starlark.AsString(items[5]); !ok {
			return nil, fmt.Errorf("dyson: %s encoding must be a string or None", responseTypeName)
		}
	}
	return &responseValue{
		statusCode: int(statusCode),
		headers:    headers,
		content:    content,
		url:        url,
		reason:     reason,
		encoding:   items[5],
	}, nil
}
