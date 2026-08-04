package dyson_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson"
	"github.com/spf13/afero"
)

type recordingCommandRunner struct {
	ctx     context.Context
	command dyson.Command
}

func (r *recordingCommandRunner) RunCommand(ctx context.Context, command dyson.Command) (dyson.CommandResult, error) {
	r.ctx = ctx
	r.command = command
	return dyson.CommandResult{Stdout: []byte("configured\n")}, nil
}

type memoryEnvironment struct {
	values map[string]string
}

func (e *memoryEnvironment) Environ() []string {
	environ := make([]string, 0, len(e.values))
	for key, value := range e.values {
		environ = append(environ, key+"="+value)
	}
	return environ
}

func (e *memoryEnvironment) LookupEnv(key string) (string, bool) {
	value, ok := e.values[key]
	return value, ok
}

func (e *memoryEnvironment) Setenv(key, value string) error {
	e.values[key] = value
	return nil
}

func (e *memoryEnvironment) Unsetenv(key string) error {
	delete(e.values, key)
	return nil
}

func (e *memoryEnvironment) UserHomeDir() (string, error) {
	return "/home/fake", nil
}

func (e *memoryEnvironment) ExpandEnv(value string) string {
	return os.Expand(value, func(key string) string { return e.values[key] })
}

func TestStdlibSelectionExposesExactSurface(t *testing.T) {
	fsys := dyson.FromIOFS(fstest.MapFS{
		"visible.txt": {Data: []byte("selected")},
	})
	stdlib := dyson.NewStdlib(dyson.StdlibConfig{FS: fsys})
	selected := stdlib.Select(dyson.StdlibSelection{
		Modules: []string{"re.star"},
		Globals: []string{"open"},
	})
	sphere := dyson.NewSphere(nil, selected)
	t.Cleanup(func() {
		be.Err(t, sphere.Close(), nil)
	})

	be.Err(t, sphere.Eval(t.Context(), `
load("re.star", "re")
if re.match("[a-z]+", "selected") == None:
    fail("selected module unavailable")
file = open("visible.txt")
if file.read() != "selected":
    fail("selected global unavailable")
file.close()
`), nil)

	err := sphere.Eval(t.Context(), `load("os.star", "os")`)
	if err == nil || !strings.Contains(err.Error(), `module "os.star" is not registered`) {
		t.Fatalf("Eval() error = %v, want unselected module rejection", err)
	}
}

func TestStdlibUsesConfiguredFileSystemAndEnvironment(t *testing.T) {
	fsys := dyson.FromIOFS(fstest.MapFS{
		"visible.txt": {Data: []byte("sandboxed")},
	})
	env := &memoryEnvironment{values: map[string]string{"DYSON_TEST": "configured"}}
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{
		FS:  fsys,
		Env: env,
	}))

	be.Err(t, sphere.Eval(t.Context(), `
load("os.star", "os")
if os.listdir(".") != ["visible.txt"]:
    fail("os did not use the configured filesystem")
if os.getenv("DYSON_TEST") != "configured":
    fail("os did not use the configured environment")
configured_file = open(".//visible.txt")
if configured_file.read() != "sandboxed":
    fail("open did not use the configured filesystem")
configured_file.close()
fd = os.open("./visible.txt", os.O_RDONLY)
if os.read(fd, 9) != b"sandboxed":
    fail("os.open did not use the configured filesystem")
os.close(fd)
`), nil)
}

func TestStdlibUsesAferoMemoryFileSystem(t *testing.T) {
	fsys := afero.NewMemMapFs()
	be.Err(t, afero.WriteFile(fsys, "input.txt", []byte("memory"), 0o600), nil)
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{FS: fsys}))
	t.Cleanup(func() {
		be.Err(t, sphere.Close(), nil)
	})

	be.Err(t, sphere.Eval(t.Context(), `
load("os.star", "os")
if open("input.txt").read() != "memory":
    fail("open did not read from MemMapFs")
os.mkdir("output")
fd = os.open("output/result.txt", os.O_CREAT | os.O_WRONLY, 0o600)
os.write(fd, b"written")
os.close(fd)
if not os.path.isfile("output/result.txt"):
    fail("os did not mutate MemMapFs")
`), nil)

	result, err := afero.ReadFile(fsys, "output/result.txt")
	be.Err(t, err, nil)
	be.Equal(t, string(result), "written")
}

type fakeClock struct {
	now      time.Time
	sleepCtx context.Context
	sleeps   []time.Duration
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, duration time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.sleepCtx = ctx
	c.sleeps = append(c.sleeps, duration)
	c.now = c.now.Add(duration)
	return nil
}

type fakeProcess struct{}

func (fakeProcess) Getpid() int                { return 42 }
func (fakeProcess) Getppid() int               { return 7 }
func (fakeProcess) Kill(pid, signal int) error { return nil }
func (fakeProcess) Getuid() int                { return 1000 }
func (fakeProcess) Geteuid() int               { return 1001 }
func (fakeProcess) Getgid() int                { return 2000 }
func (fakeProcess) Getegid() int               { return 2001 }
func (fakeProcess) Getgroups() ([]int, error)  { return []int{2000, 2002}, nil }
func (fakeProcess) Umask(mask int) int         { return 0o022 }

type fakeWorkingDirectory struct {
	path string
}

func (d *fakeWorkingDirectory) Getwd() (string, error) { return d.path, nil }
func (d *fakeWorkingDirectory) Chdir(path string) error {
	d.path = path
	return nil
}

type fakeTerminal struct{}

func (fakeTerminal) TerminalSize() (int, int, error) { return 120, 40, nil }

func TestStdlibUsesConfiguredOSCapabilities(t *testing.T) {
	workingDirectory := &fakeWorkingDirectory{path: "/virtual/work"}
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{
		Process:          fakeProcess{},
		WorkingDirectory: workingDirectory,
		Terminal:         fakeTerminal{},
		Platform: dyson.Platform{
			OSName:            "test-os",
			PathListSeparator: ":",
			DevNull:           "/dev/null",
		},
	}))

	be.Err(t, sphere.Eval(t.Context(), `
load("os.star", "os")
load("shutil.star", "shutil")
if os.getpid() != 42 or os.getppid() != 7:
    fail("os did not use the configured process")
if os.getcwd() != "/virtual/work":
    fail("os did not use the configured working directory")
if os.name != "test-os":
    fail("os did not use the configured platform")
if shutil.get_terminal_size() != (120, 40):
    fail("shutil did not use the configured terminal")
`), nil)
}

type mutableRecordingFileSystem struct {
	afero.Fs
	atime time.Time
	mtime time.Time
}

func (f *mutableRecordingFileSystem) Chtimes(_ string, atime, mtime time.Time) error {
	f.atime, f.mtime = atime, mtime
	return nil
}

func TestStdlibUsesConfiguredClockForUtime(t *testing.T) {
	fsys := &mutableRecordingFileSystem{Fs: dyson.FromIOFS(fstest.MapFS{
		"file.txt": {Data: []byte("content")},
	})}
	clock := &fakeClock{now: time.Unix(123, 456)}
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{
		FS:    fsys,
		Clock: clock,
	}))

	be.Err(t, sphere.Eval(t.Context(), `
load("os.star", "os")
os.utime("file.txt")
`), nil)
	if !fsys.atime.Equal(clock.now) || !fsys.mtime.Equal(clock.now) {
		t.Fatalf("utime = (%v, %v), want configured time %v", fsys.atime, fsys.mtime, clock.now)
	}
}

func TestStdlibUsesConfiguredClock(t *testing.T) {
	clock := &fakeClock{now: time.Unix(100, 250_000_000)}
	ctx := t.Context()
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{
		Clock: clock,
	}))

	be.Err(t, sphere.Eval(ctx, `
load("time.star", "time")
if time.time_ns() != 100250000000:
    fail("time did not use the configured clock")
if time.monotonic_ns() != 0:
    fail("monotonic clock did not use the configured origin")
time.sleep(1.5)
if time.time_ns() != 101750000000:
    fail("sleep did not advance the configured clock")
if time.monotonic_ns() != 1500000000:
    fail("monotonic clock did not advance with the configured clock")
`), nil)
	if clock.sleepCtx != ctx {
		t.Fatal("clock did not receive the evaluation context")
	}
	if !slices.Equal(clock.sleeps, []time.Duration{1500 * time.Millisecond}) {
		t.Fatalf("sleep durations = %v", clock.sleeps)
	}
}

func TestHostCommandRunnerIsExplicitOptIn(t *testing.T) {
	config := dyson.HostStdlibConfig(t.TempDir())
	if config.CommandRunner != nil {
		t.Fatal("HostStdlibConfig unexpectedly enables command execution")
	}
	config.CommandRunner = dyson.HostCommandRunner()
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(config))

	be.Err(t, sphere.Eval(t.Context(), `
load("subprocess.star", "subprocess")
result = subprocess.run("exit 0", shell=True)
if result.returncode != 0:
    fail("host command runner did not execute the command")
`), nil)
}

func TestHostStdlibConfigUsesRootAsDefaultCommandDirectory(t *testing.T) {
	root := t.TempDir()
	runner := &recordingCommandRunner{}
	config := dyson.HostStdlibConfig(root)
	config.CommandRunner = runner
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(config))

	be.Err(t, sphere.Eval(t.Context(), `
load("subprocess.star", "subprocess")
subprocess.run(["tool"])
`), nil)
	be.Equal(t, runner.command.Dir, root)

	be.Err(t, sphere.Eval(t.Context(), `
subprocess.run(["tool"], cwd="/explicit")
`), nil)
	be.Equal(t, runner.command.Dir, "/explicit")

	be.Err(t, sphere.Eval(t.Context(), `
load("os.star", "os")
os.system("tool")
`), nil)
	be.Equal(t, runner.command.Dir, root)
}

func TestStdlibWithoutClockFailsClosed(t *testing.T) {
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{}))

	err := sphere.Eval(t.Context(), `
load("time.star", "time")
time.time()
`)
	if err == nil || !strings.Contains(err.Error(), "time.time: clock is not configured") {
		t.Fatalf("Eval() error = %v, want unconfigured clock error", err)
	}
}

func TestHostStdlibConfigSharesFileSystemState(t *testing.T) {
	root := t.TempDir()
	be.Err(t, os.WriteFile(filepath.Join(root, "source.txt"), []byte("source"), 0o600), nil)
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.HostStdlibConfig(root)))

	be.Err(t, sphere.Eval(t.Context(), `
load("glob.star", "glob")
load("os.star", "os")
load("shutil.star", "shutil")
load("tempfile.star", "tempfile")
if glob.glob("*.txt") != ["source.txt"]:
    fail("glob did not use the configured filesystem")
shutil.copyfile("source.txt", "copied.txt")
if not os.path.exists("copied.txt"):
    fail("shutil and os did not share the configured filesystem")
fd, path = tempfile.mkstemp(dir=".")
os.write(fd, b"shared")
os.close(fd)
fd = os.open(path, os.O_RDONLY)
content = os.read(fd, 6)
os.close(fd)
if content != b"shared":
    fail("os and tempfile did not share file descriptors")
`), nil)
}

func TestStdlibWithoutFileSystemFailsClosed(t *testing.T) {
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{}))

	err := sphere.Eval(t.Context(), `
load("os.star", "os")
os.listdir(".")
`)
	if err == nil || !strings.Contains(err.Error(), "os.listdir: filesystem operations are not configured") {
		t.Fatalf("Eval() error = %v, want unconfigured filesystem error", err)
	}
}

func TestStdlibWithoutCommandRunnerFailsClosed(t *testing.T) {
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{}))

	err := sphere.Eval(t.Context(), `
load("subprocess.star", "subprocess")
subprocess.run(["should-not-run"])
`)
	if err == nil || !strings.Contains(err.Error(), "subprocess.run: subprocess execution is not configured") {
		t.Fatalf("Eval() error = %v, want unconfigured subprocess error", err)
	}
}

type blockingCommandRunner struct {
	started chan struct{}
}

func (r blockingCommandRunner) RunCommand(ctx context.Context, command dyson.Command) (dyson.CommandResult, error) {
	close(r.started)
	<-ctx.Done()
	return dyson.CommandResult{}, ctx.Err()
}

func TestEvalCancellationStopsConfiguredCommand(t *testing.T) {
	runner := blockingCommandRunner{started: make(chan struct{})}
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{
		CommandRunner: runner,
	}))

	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() {
		result <- sphere.Eval(ctx, `
load("subprocess.star", "subprocess")
subprocess.run(["block"])
`)
	}()

	<-runner.started
	cancel()
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
			t.Fatalf("Eval() error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Eval did not stop the active command")
	}
}

func TestStdlibUsesConfiguredCommandRunner(t *testing.T) {
	runner := &recordingCommandRunner{}
	ctx := t.Context()
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{
		CommandRunner: runner,
	}))

	be.Err(t, sphere.Eval(ctx, `
load("subprocess.star", "subprocess")
result = subprocess.run(["tool", "argument"], capture_output=True, text=True)
if result.stdout != "configured\n":
    fail("unexpected command output")
`), nil)
	if runner.ctx != ctx {
		t.Fatal("command runner did not receive the evaluation context")
	}
	if !slices.Equal(runner.command.Args, []string{"tool", "argument"}) {
		t.Fatalf("command args = %q", runner.command.Args)
	}

	be.Err(t, sphere.Eval(ctx, `
load("os.star", "os")
if os.system("shell command") != 0:
    fail("unexpected shell command status")
`), nil)
	if !runner.command.Shell || !slices.Equal(runner.command.Args, []string{"shell command"}) {
		t.Fatalf("os.system command = %#v", runner.command)
	}
}
