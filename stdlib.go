package dyson

import (
	"errors"
	"io"
	"io/fs"
	"maps"

	stdlibbuiltins "github.com/spachava753/dyson/internal/stdlib/builtins"
	stdlibglob "github.com/spachava753/dyson/internal/stdlib/glob"
	stdlibgrp "github.com/spachava753/dyson/internal/stdlib/grp"
	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	stdlibpwd "github.com/spachava753/dyson/internal/stdlib/pwd"
	stdlibre "github.com/spachava753/dyson/internal/stdlib/re"
	stdlibrequests "github.com/spachava753/dyson/internal/stdlib/requests"
	stdlibshutil "github.com/spachava753/dyson/internal/stdlib/shutil"
	stdlibsignal "github.com/spachava753/dyson/internal/stdlib/signal"
	stdlibsubprocess "github.com/spachava753/dyson/internal/stdlib/subprocess"
	stdlibtempfile "github.com/spachava753/dyson/internal/stdlib/tempfile"
	stdlibtime "github.com/spachava753/dyson/internal/stdlib/time"
	"github.com/spachava753/dyson/internal/xfs"
	"go.starlark.net/starlark"
)

var errFilesystemNotConfigured = errors.New("filesystem operations are not configured")

type unconfiguredFileSystem struct{}

// ReadDir implements FileSystem and reports that filesystem access is unavailable.
func (unconfiguredFileSystem) ReadDir(string) ([]fs.DirEntry, error) {
	return nil, errFilesystemNotConfigured
}

// Stat implements FileSystem and reports that filesystem access is unavailable.
func (unconfiguredFileSystem) Stat(string) (fs.FileInfo, error) {
	return nil, errFilesystemNotConfigured
}

// Lstat implements FileSystem and reports that filesystem access is unavailable.
func (unconfiguredFileSystem) Lstat(string) (fs.FileInfo, error) {
	return nil, errFilesystemNotConfigured
}

// Stdlib owns one configured standard library and its open host resources. It
// is a SphereSource; passing it to NewSphere transfers cleanup to that Sphere.
type Stdlib struct {
	modules   map[string]starlark.StringDict
	globals   starlark.StringDict
	resources []io.Closer
	closed    bool
}

// Modules returns a snapshot of the loadable standard-library namespaces.
func (s *Stdlib) Modules() map[string]starlark.StringDict {
	return cloneModules(s.modules)
}

// Globals returns a snapshot of the standard-library globals.
func (s *Stdlib) Globals() starlark.StringDict {
	return maps.Clone(s.globals)
}

// StdlibSelection identifies the exact standard modules and globals exposed by
// a selected Stdlib source. Unknown names are omitted, preserving fail-closed
// behavior. Module names are load names such as "os.star".
type StdlibSelection struct {
	Modules []string
	Globals []string
}

type selectedStdlib struct {
	owner   *Stdlib
	modules map[string]starlark.StringDict
	globals starlark.StringDict
}

// Select returns an owned SphereSource containing only selection. Closing the
// source closes the underlying Stdlib, so it must not be shared by active
// sessions with independent lifetimes.
func (s *Stdlib) Select(selection StdlibSelection) SphereSource {
	selected := &selectedStdlib{
		owner:   s,
		modules: map[string]starlark.StringDict{},
		globals: starlark.StringDict{},
	}
	for _, name := range selection.Modules {
		if globals, ok := s.modules[name]; ok {
			selected.modules[name] = maps.Clone(globals)
		}
	}
	for _, name := range selection.Globals {
		if value, ok := s.globals[name]; ok {
			selected.globals[name] = value
		}
	}
	return selected
}

// Modules returns a snapshot of the selected loadable namespaces.
func (s *selectedStdlib) Modules() map[string]starlark.StringDict {
	return cloneModules(s.modules)
}

// Globals returns a snapshot of the selected standard-library globals.
func (s *selectedStdlib) Globals() starlark.StringDict {
	return maps.Clone(s.globals)
}

// Close closes the Stdlib that owns the selected source.
func (s *selectedStdlib) Close() error {
	return s.owner.Close()
}

// Close releases all files owned by the standard library. It is idempotent.
func (s *Stdlib) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	resources := s.resources
	s.resources = nil

	var closeErrors []error
	for _, resource := range resources {
		if err := resource.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

// NewStdlib constructs a standard-library owner from explicit host
// capabilities. Callers must close it directly or pass it to a Sphere that owns
// the same execution lifetime.
func NewStdlib(config StdlibConfig) *Stdlib {
	fileSystem := config.FS
	if fileSystem == nil {
		fileSystem = unconfiguredFileSystem{}
	}
	fileDescriptors := xfs.NewFileDescriptors()
	files := stdlibbuiltins.NewFileRegistry(config.FS)
	commandRunner := config.configuredCommandRunner()
	osModule := stdlibos.MakeModule(stdlibos.ModuleConfig{
		FS:              fileSystem,
		Env:             config.Env,
		Process:         config.Process,
		WorkingDir:      config.WorkingDirectory,
		Platform:        config.Platform,
		CommandRunner:   commandRunner,
		Clock:           config.Clock,
		FileDescriptors: fileDescriptors,
	})
	globModule := stdlibglob.MakeModule(osModule)
	shutilModule := stdlibshutil.MakeModule(stdlibshutil.ModuleConfig{
		OS:       osModule,
		FS:       config.FS,
		Env:      config.Env,
		Terminal: config.Terminal,
		Platform: config.Platform,
	})

	return &Stdlib{
		modules: map[string]starlark.StringDict{
			stdlibglob.ModuleName + ".star": {
				stdlibglob.ModuleName: globModule,
			},
			stdlibgrp.ModuleName + ".star": {
				stdlibgrp.ModuleName: stdlibgrp.Module,
			},
			stdlibos.ModuleName + ".star": {
				stdlibos.ModuleName: osModule,
			},
			stdlibpwd.ModuleName + ".star": {
				stdlibpwd.ModuleName: stdlibpwd.Module,
			},
			stdlibre.ModuleName + ".star": {
				stdlibre.ModuleName: stdlibre.Module,
			},
			stdlibrequests.ModuleName + ".star": {
				stdlibrequests.ModuleName: stdlibrequests.MakeModule(config.HTTPClient),
			},
			stdlibshutil.ModuleName + ".star": {
				stdlibshutil.ModuleName: shutilModule,
			},
			stdlibsignal.ModuleName + ".star": {
				stdlibsignal.ModuleName: stdlibsignal.Module,
			},
			stdlibsubprocess.ModuleName + ".star": {
				stdlibsubprocess.ModuleName: stdlibsubprocess.MakeModule(commandRunner),
			},
			stdlibtempfile.ModuleName + ".star": {
				stdlibtempfile.ModuleName: stdlibtempfile.MakeModule(stdlibtempfile.ModuleConfig{
					FS:              config.FS,
					Env:             config.Env,
					FileDescriptors: fileDescriptors,
				}),
			},
			stdlibtime.ModuleName + ".star": {
				stdlibtime.ModuleName: stdlibtime.MakeModule(config.Clock),
			},
		},
		globals:   files.Globals(),
		resources: []io.Closer{files, fileDescriptors},
	}
}
