//go:build linux

package docker_test

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// These tests launch only a synthetic process in t.TempDir, never wzap or a DB.
// Set WZAP_TEST_AIR_NATIVE=1 to reproduce the unprotected Air characterization.
func TestDevReload(t *testing.T) {
	air := os.Getenv("WZAP_TEST_AIR_BIN")
	if air == "" {
		t.Skip("set WZAP_TEST_AIR_BIN to the real Air v1.61.7 binary")
	}
	version, err := exec.Command(air, "-v").CombinedOutput()
	if err != nil || !strings.Contains(string(version), "v1.61.7") {
		t.Fatalf("Air v1.61.7 required: %s: %v", version, err)
	}
	t.Run("PipeOutputReloadAndRecovery", func(t *testing.T) {
		if os.Getenv("WZAP_TEST_AIR_NATIVE") == "1" {
			t.Skip("native lifecycle characterization uses append capture; pipe output requires runtime protection")
		}
		// A single CPU makes the competing PTY-copy readers take the adverse
		// lock order reliably, as can happen in a CPU-limited dev container.
		t.Setenv("GOMAXPROCS", "1")
		f := newReloadFixture(t, air, "50ms")
		f.waitEvent("start", "one", 20*time.Second)
		f.source("pipe-reload")
		f.waitEvent("start", "pipe-reload", 12*time.Second)
		f.write("main.go", "package main\ninvalid Go\n")
		f.waitLog("failed to build", 12*time.Second)
		f.source("pipe-recovered")
		f.waitEvent("start", "pipe-recovered", 12*time.Second)
		f.stop()
		f.assertLifecycle()
	})
	t.Run("Drain", func(t *testing.T) {
		f := newReloadFixture(t, air, "8s")
		f.waitEvent("start", "one", 20*time.Second)
		f.source("two")
		f.waitEvent("start", "two", 30*time.Second)
		f.stop()
		f.assertLifecycle()
	})
	t.Run("DrainAndSuccessiveEdits", func(t *testing.T) {
		f := newReloadFixture(t, air, "8s")
		f.waitEvent("start", "one", 20*time.Second)
		f.source("two")
		f.waitEvent("signal", "one", 10*time.Second)
		f.source("three")
		time.Sleep(350 * time.Millisecond)
		f.source("latest")
		f.waitEvent("start", "latest", 35*time.Second)
		f.stop()
		f.assertLifecycle()
	})
	t.Run("BuildRecoveryAndWatchScope", func(t *testing.T) {
		f := newReloadFixture(t, air, "50ms")
		f.waitEvent("start", "one", 20*time.Second)
		before := f.buildCount()
		for _, name := range []string{"manager/page.vue", "manager/style.css", "manager/node_modules/ignored.go", "manager/.nuxt/ignored.go", "main_test.go", "docs/readme.html", ".worktrees/ignored/main.go", "tmp/ignored.go", ".superpowers/state.go"} {
			f.write(name, "ignored")
		}
		time.Sleep(time.Second)
		if f.buildCount() != before {
			t.Error("frontend/test/cache edits triggered Go build")
		}
		f.write("main.go", "package main\ninvalid Go\n")
		f.waitLog("failed to build", 10*time.Second)
		f.source("recovered")
		f.waitEvent("start", "recovered", 20*time.Second)
		for _, name := range []string{"go.mod", "go.sum", "internal/storage/migrations/001.sql", "docs/docs.go", "manager/handler.go"} {
			starts := f.count("start")
			content := "// changed\n"
			if name == "go.mod" {
				content = "module fixture\n\ngo 1.26\n// changed\n"
			}
			if name == "go.sum" {
				content = "\n"
			}
			if name == "manager/handler.go" {
				content = "package manager\n"
			}
			if name == "docs/docs.go" {
				content = "package docs\n"
			}
			f.write(name, content)
			f.wait(func() bool { return f.count("start") > starts }, 20*time.Second, "restart for "+name)
		}
		f.stop()
		f.assertLifecycle()
	})
	t.Run("StopDuringDrain", func(t *testing.T) {
		f := newReloadFixture(t, air, "8s")
		f.waitEvent("start", "one", 20*time.Second)
		f.source("two")
		f.waitEvent("signal", "one", 10*time.Second)
		f.stop()
		f.assertLifecycle()
	})
	t.Run("SupervisorStop", func(t *testing.T) {
		f := newReloadFixture(t, air, "8s")
		f.waitEvent("start", "one", 20*time.Second)
		f.stop()
		f.assertLifecycle()
	})
}

// Check the parent forwarding path without Air's own one-shot signal handler
// concealing a repeated delivery. The controlled child acts only as Air here.
func TestDevSupervisorSignalOnce(t *testing.T) {
	f := &reloadFixture{t: t, dir: t.TempDir(), done: make(chan error, 1), drain: 200 * time.Millisecond}
	script, err := os.ReadFile("dev-go.sh")
	if err != nil {
		t.Fatal(err)
	}
	f.write("docker/dev-go.sh", string(script))
	f.source("synthetic-air")
	build := exec.Command("go", "build", "-o", "synthetic-air", "main.go")
	build.Dir = f.dir
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build synthetic Air: %s: %v", output, err)
	}
	f.cmd = exec.Command("bash", "docker/dev-go.sh", "./synthetic-air")
	f.cmd.Dir = f.dir
	f.cmd.Env = append(os.Environ(), "FIXTURE_DRAIN=200ms")
	f.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := f.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { f.done <- f.cmd.Wait() }()
	t.Cleanup(func() {
		if !f.stopped {
			_ = syscall.Kill(-f.cmd.Process.Pid, syscall.SIGKILL)
			<-f.done
		}
	})
	f.waitEvent("start", "synthetic-air", 10*time.Second)
	if err := f.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	f.waitEvent("signal", "synthetic-air", time.Second)
	// A second parent signal while its exact Air child drains must stay local.
	f.stop()
	f.assertLifecycle()
}

type reloadEvent struct {
	Kind, Version string
	PID           int
	At            int64
}
type reloadFixture struct {
	t              *testing.T
	dir            string
	cmd            *exec.Cmd
	done           chan error
	stopped        bool
	supervisorExit int64
	drain          time.Duration
}

func newReloadFixture(t *testing.T, air, drain string) *reloadFixture {
	t.Helper()
	f := &reloadFixture{t: t, dir: t.TempDir(), done: make(chan error, 1)}
	f.drain, _ = time.ParseDuration(drain)
	config, err := os.ReadFile("../.air.toml")
	if err != nil {
		t.Fatal(err)
	}
	set := func(key, value string) {
		re := regexp.MustCompile(`(?m)^` + key + ` = .*`)
		config = re.ReplaceAll(config, []byte(key+" = "+value))
	}
	set("cmd", `"go build -o ./tmp/wzap ./main.go"`)
	native := os.Getenv("WZAP_TEST_AIR_NATIVE") == "1"
	if native {
		set("full_bin", `"exec ./tmp/wzap serve"`)
		set("kill_delay", `"12s"`)
		set("clean_on_exit", "true")
		set("include_ext", `["go", "mod", "sum", "sql"]`)
		set("exclude_dir", `["tmp", "vendor", "node_modules", ".git", "manager/node_modules", "manager/.nuxt", "manager/.output", "manager/.data", ".worktrees", ".superpowers"]`)
	}
	f.write(".air.toml", string(config))
	f.write("go.mod", "module fixture\n\ngo 1.26\n")
	f.write("go.sum", "")
	f.write("internal/storage/migrations/001.sql", "-- initial\n")
	f.write("docs/docs.go", "package docs\n")
	f.write("manager/handler.go", "package manager\n")
	f.source("one")
	f.cmd = exec.Command(air, "-c", ".air.toml")
	if !native {
		script, err := os.ReadFile("dev-go.sh")
		if err != nil {
			t.Fatal(err)
		}
		f.write("docker/dev-go.sh", string(script))
		f.cmd = exec.Command("bash", "docker/dev-go.sh", air, "-c", ".air.toml")
	}
	f.cmd.Dir = f.dir
	f.cmd.Env = append(os.Environ(), "FIXTURE_DRAIN="+drain)
	f.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	log, err := os.OpenFile(filepath.Join(f.dir, "air.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	f.cmd.Stdout = log
	f.cmd.Stderr = log
	var pipes []*os.File
	var copied []chan error
	if !native {
		// Match Docker's separate output pipes. Only the collector's file has
		// O_APPEND; Air must get pipe descriptors so runtime protection, rather
		// than fixture-only flags, prevents its shared-PTY zero-copy deadlock.
		for _, output := range []*io.Writer{&f.cmd.Stdout, &f.cmd.Stderr} {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
			*output = writer
			pipes = append(pipes, writer)
			done := make(chan error, 1)
			copied = append(copied, done)
			go func() {
				_, err := io.Copy(log, reader)
				_ = reader.Close()
				done <- err
			}()
		}
	}
	if err := f.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	for _, writer := range pipes {
		_ = writer.Close()
	}
	go func() {
		err := f.cmd.Wait()
		for _, done := range copied {
			if copyErr := <-done; err == nil {
				err = copyErr
			}
		}
		_ = log.Close()
		f.done <- err
	}()
	t.Cleanup(func() {
		if !f.stopped {
			_ = f.cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-f.done:
			case <-time.After(15 * time.Second):
				_ = syscall.Kill(-f.cmd.Process.Pid, syscall.SIGKILL)
			}
		}
		for _, e := range f.events() {
			if e.Kind == "start" {
				_ = syscall.Kill(e.PID, syscall.SIGKILL)
			}
		}
		if t.Failed() {
			data, _ := os.ReadFile(filepath.Join(f.dir, "air.log"))
			t.Logf("Air output:\n%s", data)
		}
	})
	return f
}
func (f *reloadFixture) write(name, value string) {
	f.t.Helper()
	p := filepath.Join(f.dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(value), 0644); err != nil {
		f.t.Fatal(err)
	}
}
func (f *reloadFixture) source(version string) {
	f.write("main.go", strings.ReplaceAll(reloadChildSource, "VERSION", strconv.Quote(version)))
}
func (f *reloadFixture) events() []reloadEvent {
	data, _ := os.ReadFile(filepath.Join(f.dir, "events.jsonl"))
	var events []reloadEvent
	for _, line := range strings.Split(string(data), "\n") {
		var e reloadEvent
		if json.Unmarshal([]byte(line), &e) == nil {
			events = append(events, e)
		}
	}
	return events
}
func (f *reloadFixture) count(kind string) int {
	n := 0
	for _, e := range f.events() {
		if e.Kind == kind {
			n++
		}
	}
	return n
}
func (f *reloadFixture) wait(check func() bool, timeout time.Duration, label string) {
	f.t.Helper()
	until := time.Now().Add(timeout)
	for time.Now().Before(until) {
		if check() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	f.t.Fatalf("timeout waiting for %s; events=%+v", label, f.events())
}
func (f *reloadFixture) waitEvent(kind, version string, timeout time.Duration) {
	f.t.Helper()
	f.wait(func() bool {
		for _, e := range f.events() {
			if e.Kind == kind && e.Version == version {
				return true
			}
		}
		return false
	}, timeout, kind+" "+version)
	if kind == "start" {
		time.Sleep(100 * time.Millisecond)
	} // Air registers its child immediately after spawning it.
}
func (f *reloadFixture) waitLog(value string, timeout time.Duration) {
	f.t.Helper()
	f.wait(func() bool {
		b, _ := os.ReadFile(filepath.Join(f.dir, "air.log"))
		return strings.Contains(string(b), value)
	}, timeout, value)
}
func (f *reloadFixture) stop() {
	f.t.Helper()
	start := time.Now()
	if err := f.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		f.t.Fatal(err)
	}
	select {
	case err := <-f.done:
		f.stopped = true
		f.supervisorExit = time.Now().UnixNano()
		f.t.Logf("supervisor exit after %s (%v)", time.Since(start), err)
		if err != nil {
			f.t.Errorf("supervisor exit: %v", err)
		}
	case <-time.After(20 * time.Second):
		f.t.Fatal("supervisor exceeded 20s stop grace")
	}
}
func (f *reloadFixture) assertLifecycle() {
	f.t.Helper()
	active := map[int]bool{}
	signals := map[int]int{}
	signaledAt := map[int]int64{}
	started := map[int]bool{}
	events := f.events()
	if len(events) == 0 {
		f.t.Fatal("no child events")
	}
	base := events[0].At
	for _, e := range events {
		f.t.Logf("+%.3fs %s version=%s pid=%d", float64(e.At-base)/1e9, e.Kind, e.Version, e.PID)
		switch e.Kind {
		case "start":
			if len(active) > 0 {
				f.t.Errorf("replacement %d started before prior children exited: %v", e.PID, active)
			}
			active[e.PID] = true
			started[e.PID] = true
		case "signal":
			signals[e.PID]++
			if signals[e.PID] == 1 {
				signaledAt[e.PID] = e.At
			}
			if signals[e.PID] > 1 {
				f.t.Errorf("child %d received repeated termination signals", e.PID)
			}
		case "exit":
			if elapsed := time.Duration(e.At - signaledAt[e.PID]); elapsed < f.drain {
				f.t.Errorf("child %d drain shortened to %s", e.PID, elapsed)
			}
			delete(active, e.PID)
			if e.At > f.supervisorExit {
				f.t.Errorf("child %d exited after supervisor", e.PID)
			}
		}
	}
	if len(active) > 0 {
		f.t.Errorf("supervisor exited with active children: %v", active)
	}
	for pid := range started {
		n := signals[pid]
		if n != 1 {
			f.t.Errorf("child %d signal count=%d", pid, n)
		}
		// Air reaps its exact child; a zombie is also evidence of bad cleanup.
		f.wait(func() bool { return syscall.Kill(pid, 0) == syscall.ESRCH }, 3*time.Second, "child reaped")
	}
}

func (f *reloadFixture) buildCount() int {
	b, _ := os.ReadFile(filepath.Join(f.dir, "air.log"))
	return strings.Count(string(b), "building...")
}

const reloadChildSource = `package main
import("encoding/json";"os";"os/signal";"syscall";"time")
func main(){
 f,err:=os.OpenFile("events.jsonl",os.O_CREATE|os.O_WRONLY|os.O_APPEND,0644);if err!=nil{panic(err)};defer f.Close()
 record:=func(kind string){b,_:=json.Marshal(struct{Kind,Version string;PID int;At int64}{kind,VERSION,os.Getpid(),time.Now().UnixNano()});b=append(b,'\n');if _,err:=f.Write(b);err!=nil{panic(err)}}
 ch:=make(chan os.Signal,8);signal.Notify(ch,syscall.SIGINT,syscall.SIGTERM)
 record("start");<-ch;record("signal");drain,_:=time.ParseDuration(os.Getenv("FIXTURE_DRAIN"));timer:=time.NewTimer(drain)
 for{select{case <-ch:record("signal");case <-timer.C:record("exit");return}}
}
`
