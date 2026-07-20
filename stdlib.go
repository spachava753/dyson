package dyson

import (
	"errors"
	"io/fs"

	stdlibglob "github.com/spachava753/dyson/internal/stdlib/glob"
	stdlibgrp "github.com/spachava753/dyson/internal/stdlib/grp"
	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	stdlibpwd "github.com/spachava753/dyson/internal/stdlib/pwd"
	stdlibre "github.com/spachava753/dyson/internal/stdlib/re"
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

func (unconfiguredFileSystem) ReadDir(string) ([]fs.DirEntry, error) {
	return nil, errFilesystemNotConfigured
}

func (unconfiguredFileSystem) Stat(string) (fs.FileInfo, error) {
	return nil, errFilesystemNotConfigured
}

func (unconfiguredFileSystem) Lstat(string) (fs.FileInfo, error) {
	return nil, errFilesystemNotConfigured
}

// StdlibModules constructs Dyson's loadable standard-library compatibility
// modules from explicit host capabilities. Each call creates fresh module state
// and one descriptor table shared by os and tempfile; the configured filesystem
// and environment are shared by every module that consumes them.
func StdlibModules(config StdlibConfig) map[string]starlark.StringDict {
	fileSystem := config.FS
	if fileSystem == nil {
		fileSystem = unconfiguredFileSystem{}
	}
	fileDescriptors := xfs.NewFileDescriptors()
	osModule := stdlibos.MakeModule(stdlibos.ModuleConfig{
		FS:              fileSystem,
		Env:             config.Env,
		Process:         config.Process,
		WorkingDir:      config.WorkingDirectory,
		Platform:        config.Platform,
		CommandRunner:   config.CommandRunner,
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

	return map[string]starlark.StringDict{
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
		stdlibshutil.ModuleName + ".star": {
			stdlibshutil.ModuleName: shutilModule,
		},
		stdlibsignal.ModuleName + ".star": {
			stdlibsignal.ModuleName: stdlibsignal.Module,
		},
		stdlibsubprocess.ModuleName + ".star": {
			stdlibsubprocess.ModuleName: stdlibsubprocess.MakeModule(config.CommandRunner),
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
	}
}
