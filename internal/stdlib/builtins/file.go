package builtins

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"unicode/utf8"

	"github.com/spachava753/dyson/internal/pybytes"
	"github.com/spachava753/dyson/internal/xfs"
	"go.starlark.net/starlark"
)

var fileMethods = map[string]*starlark.Builtin{
	"close": starlark.NewBuiltin("file.close", fileClose),
	"read":  starlark.NewBuiltin("file.read", fileRead),
}

// FileRegistry provides the global open builtin and owns its live handles.
// It is not safe for concurrent use.
type FileRegistry struct {
	fsys   xfs.FS
	files  map[*fileValue]struct{}
	closed bool
}

// NewFileRegistry returns an open-file registry backed by fsys.
func NewFileRegistry(fsys xfs.FS) *FileRegistry {
	return &FileRegistry{
		fsys:  fsys,
		files: map[*fileValue]struct{}{},
	}
}

// Globals returns Python-inspired globals backed by the registry.
func (f *FileRegistry) Globals() starlark.StringDict {
	return starlark.StringDict{
		"open": starlark.NewBuiltin("open", f.open),
	}
}

// Close closes all live handles. It is idempotent.
func (f *FileRegistry) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	files := f.files
	f.files = nil

	var closeErrors []error
	for file := range files {
		if err := file.close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close %q: %w", file.path, err))
		}
	}
	return errors.Join(closeErrors...)
}

// open validates the supported Python open signature, constructs any text
// decoder before acquiring the file, and registers the resulting handle for
// session cleanup.
func (f *FileRegistry) open(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	mode := "r"
	bufferingValue := starlark.MakeInt(-1)
	var encodingValue starlark.Value = starlark.None
	var errorsValue starlark.Value = starlark.None
	var newlineValue starlark.Value = starlark.None
	closefd := true
	var opener starlark.Value = starlark.None
	if err := starlark.UnpackArgs(
		fn.Name(), args, kwargs,
		"file", &path,
		"mode?", &mode,
		"buffering?", &bufferingValue,
		"encoding?", &encodingValue,
		"errors?", &errorsValue,
		"newline?", &newlineValue,
		"closefd?", &closefd,
		"opener?", &opener,
	); err != nil {
		return nil, err
	}
	if path == "" {
		return nil, fmt.Errorf("%s(%q): %w", fn.Name(), path, fs.ErrNotExist)
	}

	buffering, ok := bufferingValue.Int64()
	if !ok {
		return nil, fmt.Errorf("%s(%q): buffering is out of range", fn.Name(), path)
	}
	if buffering != -1 {
		return nil, fmt.Errorf("%s(%q): buffering values other than -1 are not supported", fn.Name(), path)
	}
	if !closefd {
		return nil, fmt.Errorf("%s(%q): closefd=False is not supported for file paths", fn.Name(), path)
	}
	if opener != starlark.None {
		return nil, fmt.Errorf("%s(%q): opener is not supported", fn.Name(), path)
	}

	binary := false
	switch mode {
	case "r", "rt":
	case "rb":
		binary = true
	default:
		return nil, fmt.Errorf("%s(%q): unsupported mode %q; supported modes are \"r\", \"rt\", and \"rb\"", fn.Name(), path, mode)
	}

	if binary {
		if encodingValue != starlark.None {
			return nil, fmt.Errorf("%s(%q): binary mode does not take an encoding argument", fn.Name(), path)
		}
		if errorsValue != starlark.None {
			return nil, fmt.Errorf("%s(%q): binary mode does not take an errors argument", fn.Name(), path)
		}
		if newlineValue != starlark.None {
			return nil, fmt.Errorf("%s(%q): binary mode does not take a newline argument", fn.Name(), path)
		}
	}

	encoding := "utf-8"
	if encodingValue != starlark.None {
		value, ok := starlark.AsString(encodingValue)
		if !ok {
			return nil, fmt.Errorf("%s(%q): encoding must be a string or None", fn.Name(), path)
		}
		encoding = value
	}
	errors := "strict"
	if errorsValue != starlark.None {
		value, ok := starlark.AsString(errorsValue)
		if !ok {
			return nil, fmt.Errorf("%s(%q): errors must be a string or None", fn.Name(), path)
		}
		errors = value
	}
	universalNewlines := newlineValue == starlark.None
	if newlineValue != starlark.None {
		newline, ok := starlark.AsString(newlineValue)
		if !ok {
			return nil, fmt.Errorf("%s(%q): newline must be a string or None", fn.Name(), path)
		}
		if newline != "" && newline != "\n" && newline != "\r" && newline != "\r\n" {
			return nil, fmt.Errorf("%s(%q): illegal newline value %q", fn.Name(), path, newline)
		}
	}

	var decoder *pybytes.Decoder
	if !binary {
		var err error
		decoder, err = pybytes.NewDecoder(encoding, errors)
		if err != nil {
			return nil, fmt.Errorf("%s(%q): %w", fn.Name(), path, err)
		}
	}

	file, err := f.openRead(path)
	if err != nil {
		return nil, fmt.Errorf("%s(%q): %w", fn.Name(), path, err)
	}
	value := &fileValue{
		owner:             f,
		file:              file,
		path:              path,
		mode:              mode,
		binary:            binary,
		decoder:           decoder,
		universalNewlines: universalNewlines,
	}
	f.files[value] = struct{}{}
	return value, nil
}

func (f *FileRegistry) openRead(path string) (xfs.ReadFile, error) {
	if f.closed {
		return nil, fmt.Errorf("file session is closed")
	}
	if f.fsys == nil {
		return nil, fmt.Errorf("filesystem access is not configured")
	}
	fsys, ok := f.fsys.(xfs.ReadFS)
	if !ok {
		return nil, fmt.Errorf("filesystem does not support file reading")
	}
	return fsys.OpenRead(path)
}

type fileValue struct {
	owner              *FileRegistry
	file               xfs.ReadFile
	path               string
	mode               string
	binary             bool
	decoder            *pybytes.Decoder
	universalNewlines  bool
	skipLF             bool
	closed             bool
	eof                bool
	textBuffer         []byte
	textOffset         int
	bufferedCharacters int64
}

// String returns a diagnostic representation of the open file.
func (f *fileValue) String() string {
	return fmt.Sprintf("<open file %q, mode %q>", f.path, f.mode)
}

// Type returns the Starlark type name for open files.
func (f *fileValue) Type() string { return "file" }

// Freeze implements starlark.Value; freezing does not close or otherwise alter the file.
func (f *fileValue) Freeze() {}

// Truth reports all open file values as true.
func (f *fileValue) Truth() starlark.Bool { return starlark.True }

// Hash returns an identity-based hash for the file value.
func (f *fileValue) Hash() (uint32, error) {
	return starlark.String(fmt.Sprintf("%p", f)).Hash()
}

// Attr returns a bound file method or nil for an unknown attribute.
func (f *fileValue) Attr(name string) (starlark.Value, error) {
	if method, ok := fileMethods[name]; ok {
		return method.BindReceiver(f), nil
	}
	return nil, nil
}

// AttrNames returns the attributes available on an open file.
func (f *fileValue) AttrNames() []string {
	return []string{"close", "read"}
}

// fileRead validates Python read sizing and returns native bytes or decoded text.
// Binary reads accept only -1 as a negative size; text reads retain decoded
// overflow so character counts remain exact across calls.
func fileRead(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	file, err := fileReceiver(fn)
	if err != nil {
		return nil, err
	}
	var sizeValue starlark.Value = starlark.None
	if err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 0, &sizeValue); err != nil {
		return nil, err
	}
	if file.closed {
		return nil, fmt.Errorf("%s: %q: I/O operation on closed file", fn.Name(), file.path)
	}
	size := int64(-1)
	if sizeValue != starlark.None {
		sizeInt, ok := sizeValue.(starlark.Int)
		if !ok {
			return nil, fmt.Errorf("%s: %q: size must be an int or None", fn.Name(), file.path)
		}
		var fits bool
		size, fits = sizeInt.Int64()
		if !fits {
			return nil, fmt.Errorf("%s: %q: size is out of range", fn.Name(), file.path)
		}
	}

	if file.binary && size < -1 {
		return nil, fmt.Errorf("%s: %q: read length must be non-negative or -1", fn.Name(), file.path)
	}
	if size == 0 {
		if file.binary {
			return pybytes.New(nil), nil
		}
		return starlark.String(""), nil
	}
	if file.binary {
		var reader io.Reader = file.file
		if size >= 0 {
			reader = io.LimitReader(reader, size)
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("%s: %q: %w", fn.Name(), file.path, err)
		}
		return pybytes.New(content), nil
	}

	if err := file.fillText(size); err != nil {
		return nil, fmt.Errorf("%s: %q: %w", fn.Name(), file.path, err)
	}
	return starlark.String(file.takeBufferedText(size)), nil
}

// fillText reads and decodes bounded chunks until EOF or until size characters
// are buffered. Every decoded chunk is retained before a later I/O or decoder
// error is returned, allowing subsequent reads to consume that buffered text.
func (f *fileValue) fillText(size int64) error {
	const maxChunk = 4096
	noProgress := 0
	for !f.eof {
		buffered := f.bufferedCharacters
		if size >= 0 && buffered >= size {
			return nil
		}

		chunkSize := maxChunk
		if size >= 0 {
			remaining := size - buffered
			maxWidth := int64(f.decoder.MaxBytesPerCharacter())
			if remaining <= int64(maxChunk)/maxWidth {
				chunkSize = int(remaining * maxWidth)
			}
		}
		buffer := make([]byte, chunkSize)
		n, readErr := f.file.Read(buffer)
		if n > 0 {
			decoded, err := f.decoder.Decode(buffer[:n], false)
			if err != nil {
				return err
			}
			f.appendDecoded(decoded)
			noProgress = 0
		} else if readErr == nil {
			noProgress++
			if noProgress >= 100 {
				return io.ErrNoProgress
			}
		}
		if readErr == nil {
			continue
		}
		if readErr != io.EOF {
			return readErr
		}
		decoded, err := f.decoder.Decode(nil, true)
		if err != nil {
			return err
		}
		f.appendDecoded(decoded)
		f.eof = true
	}
	return nil
}

func (f *fileValue) appendDecoded(decoded string) {
	if f.textOffset > 0 {
		copy(f.textBuffer, f.textBuffer[f.textOffset:])
		f.textBuffer = f.textBuffer[:len(f.textBuffer)-f.textOffset]
		f.textOffset = 0
	}
	if !f.universalNewlines {
		f.textBuffer = append(f.textBuffer, decoded...)
		f.bufferedCharacters += int64(utf8.RuneCountInString(decoded))
		return
	}
	for _, character := range decoded {
		if f.skipLF {
			f.skipLF = false
			if character == '\n' {
				continue
			}
		}
		if character == '\r' {
			f.textBuffer = append(f.textBuffer, '\n')
			f.skipLF = true
		} else {
			f.textBuffer = utf8.AppendRune(f.textBuffer, character)
		}
		f.bufferedCharacters++
	}
}

func (f *fileValue) takeBufferedText(size int64) string {
	buffer := f.textBuffer[f.textOffset:]
	if size < 0 {
		f.textBuffer = nil
		f.textOffset = 0
		f.bufferedCharacters = 0
		return string(buffer)
	}
	end := 0
	taken := int64(0)
	for taken < size {
		if end == len(buffer) {
			break
		}
		_, width := utf8.DecodeRune(buffer[end:])
		end += width
		taken++
	}
	text := string(buffer[:end])
	f.textOffset += end
	f.bufferedCharacters -= taken
	if f.textOffset == len(f.textBuffer) {
		f.textBuffer = nil
		f.textOffset = 0
		f.bufferedCharacters = 0
	}
	return text
}

func (f *fileValue) close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	if f.owner != nil {
		delete(f.owner.files, f)
	}
	return f.file.Close()
}

func fileClose(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	file, err := fileReceiver(fn)
	if err != nil {
		return nil, err
	}
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	if err := file.close(); err != nil {
		return nil, fmt.Errorf("%s: %q: %w", fn.Name(), file.path, err)
	}
	return starlark.None, nil
}

func fileReceiver(fn *starlark.Builtin) (*fileValue, error) {
	file, ok := fn.Receiver().(*fileValue)
	if !ok {
		return nil, fmt.Errorf("%s: receiver is %T, want file", fn.Name(), fn.Receiver())
	}
	return file, nil
}
