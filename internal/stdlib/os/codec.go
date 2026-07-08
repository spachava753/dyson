package os

import (
	"fmt"
	"io/fs"

	"github.com/spachava753/dyson/internal/codec"
	"go.starlark.net/starlark"
)

const (
	statResultTypeName = "os.stat_result"
	dirEntryTypeName   = "os.DirEntry"
)

// RegisterCodecs installs durable serialization support for os custom values.
func RegisterCodecs(registry codec.Registry) {
	registry.Register(codec.ValueCodec{
		Type:    statResultTypeName,
		Version: 1,
		Serialize: func(val starlark.Value) (codec.SerializedVal, error) {
			s, ok := val.(*statResultValue)
			if !ok {
				return codec.SerializedVal{}, fmt.Errorf("dyson: got %T for os.stat_result codec", val)
			}
			items := starlark.Tuple{
				starlark.MakeInt64(s.mode),
				starlark.MakeInt64(s.size),
				starlark.Float(s.mtime),
				starlark.Float(s.atime),
				starlark.Float(s.ctime),
				starlark.MakeInt64(s.ino),
				starlark.MakeInt64(s.dev),
				starlark.Bool(s.isDir),
			}
			payload, err := registry.Serialize(items)
			if err != nil {
				return codec.SerializedVal{}, err
			}
			return codec.SerializedVal{Type: statResultTypeName, List: []codec.SerializedVal{payload}}, nil
		},
		Restore: func(val codec.SerializedVal) (starlark.Value, error) {
			if len(val.List) != 1 {
				return nil, fmt.Errorf("dyson: invalid os.stat_result payload length %d", len(val.List))
			}
			restored, err := registry.Restore(val.List[0])
			if err != nil {
				return nil, err
			}
			items, ok := restored.(starlark.Tuple)
			if !ok || len(items) != 8 {
				return nil, fmt.Errorf("dyson: os.stat_result payload must restore to 8-item tuple")
			}
			mode, err := restoreInt64(items[0], "st_mode")
			if err != nil {
				return nil, err
			}
			size, err := restoreInt64(items[1], "st_size")
			if err != nil {
				return nil, err
			}
			mtime, err := restoreFloat(items[2], "st_mtime")
			if err != nil {
				return nil, err
			}
			atime, err := restoreFloat(items[3], "st_atime")
			if err != nil {
				return nil, err
			}
			ctime, err := restoreFloat(items[4], "st_ctime")
			if err != nil {
				return nil, err
			}
			ino, err := restoreInt64(items[5], "st_ino")
			if err != nil {
				return nil, err
			}
			dev, err := restoreInt64(items[6], "st_dev")
			if err != nil {
				return nil, err
			}
			isDir, ok := items[7].(starlark.Bool)
			if !ok {
				return nil, fmt.Errorf("dyson: os.stat_result is_dir must restore to bool, got %s", items[7].Type())
			}
			return &statResultValue{mode: mode, size: size, mtime: mtime, atime: atime, ctime: ctime, ino: ino, dev: dev, isDir: bool(isDir)}, nil
		},
	})

	registry.Register(codec.ValueCodec{
		Type:    dirEntryTypeName,
		Version: 1,
		Serialize: func(val starlark.Value) (codec.SerializedVal, error) {
			d, ok := val.(*dirEntryValue)
			if !ok {
				return codec.SerializedVal{}, fmt.Errorf("dyson: got %T for os.DirEntry codec", val)
			}
			stat, err := registry.Serialize(d.stat)
			if err != nil {
				return codec.SerializedVal{}, err
			}
			items := starlark.Tuple{
				starlark.String(d.dir),
				starlark.String(d.name),
				starlark.MakeInt64(int64(d.mode)),
			}
			metadata, err := registry.Serialize(items)
			if err != nil {
				return codec.SerializedVal{}, err
			}
			return codec.SerializedVal{Type: dirEntryTypeName, List: []codec.SerializedVal{metadata, stat}}, nil
		},
		Restore: func(val codec.SerializedVal) (starlark.Value, error) {
			if len(val.List) != 2 {
				return nil, fmt.Errorf("dyson: invalid os.DirEntry payload length %d", len(val.List))
			}
			restoredMetadata, err := registry.Restore(val.List[0])
			if err != nil {
				return nil, err
			}
			metadata, ok := restoredMetadata.(starlark.Tuple)
			if !ok || len(metadata) != 3 {
				return nil, fmt.Errorf("dyson: os.DirEntry metadata must restore to 3-item tuple")
			}
			dir, ok := starlark.AsString(metadata[0])
			if !ok {
				return nil, fmt.Errorf("dyson: os.DirEntry dir must restore to string, got %s", metadata[0].Type())
			}
			name, ok := starlark.AsString(metadata[1])
			if !ok {
				return nil, fmt.Errorf("dyson: os.DirEntry name must restore to string, got %s", metadata[1].Type())
			}
			mode, err := restoreInt64(metadata[2], "mode")
			if err != nil {
				return nil, err
			}
			restoredStat, err := registry.Restore(val.List[1])
			if err != nil {
				return nil, err
			}
			stat, ok := restoredStat.(*statResultValue)
			if !ok {
				return nil, fmt.Errorf("dyson: os.DirEntry stat must restore to os.stat_result, got %s", restoredStat.Type())
			}
			return &dirEntryValue{dir: dir, name: name, mode: fs.FileMode(mode), stat: stat}, nil
		},
	})
}

func restoreInt64(value starlark.Value, name string) (int64, error) {
	intValue, ok := value.(starlark.Int)
	if !ok {
		return 0, fmt.Errorf("dyson: %s must restore to int, got %s", name, value.Type())
	}
	converted, ok := intValue.Int64()
	if !ok {
		return 0, fmt.Errorf("dyson: %s is out of range", name)
	}
	return converted, nil
}

func restoreFloat(value starlark.Value, name string) (float64, error) {
	floatValue, ok := value.(starlark.Float)
	if !ok {
		return 0, fmt.Errorf("dyson: %s must restore to float, got %s", name, value.Type())
	}
	return float64(floatValue), nil
}
