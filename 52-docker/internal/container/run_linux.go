//go:build linux

package container

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"ccrun/internal/registry"
)

// Run starts the container and returns its exit code. It re-executes
// /proc/self/exe in new UTS, PID, mount and user namespaces; that copy runs
// Child, which finishes the setup and execs the real command.
func Run(spec Spec, rootfs string, cfg registry.ImageConfig) (int, error) {
	resolved := Resolve(spec, cfg)
	if len(resolved.Argv) == 0 {
		return 1, errors.New("no command to run (image has no entrypoint/cmd and none was given)")
	}

	absRoot, err := filepath.Abs(rootfs)
	if err != nil {
		return 1, fmt.Errorf("resolve rootfs path: %w", err)
	}
	if fi, err := os.Stat(absRoot); err != nil || !fi.IsDir() {
		return 1, fmt.Errorf("rootfs %q is not a directory", absRoot)
	}

	childArgs := append([]string{ReexecArg, absRoot, resolved.Hostname, resolved.WorkingDir, "--"}, resolved.Argv...)
	cmd := exec.Command("/proc/self/exe", childArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(resolved.Env) > 0 {
		cmd.Env = resolved.Env
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUSER,
		Unshareflags: syscall.CLONE_NEWNS,
		// Root inside the container is the invoking user outside it, so no
		// real privileges are needed.
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
		GidMappingsEnableSetgroups: false,
	}

	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("start container process: %w", err)
	}

	// Best effort: rootless setups often can't create cgroups.
	cleanup := applyCgroup(cmd.Process.Pid, spec.Memory, spec.CPUs)
	defer cleanup()

	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("container process failed: %w", err)
	}
	return 0, nil
}

// Child runs inside the new namespaces: it sets the hostname, switches root to
// the rootfs, mounts /proc and execs the command. It does not return.
func Child() {
	// Args are laid out by Run: exe ReexecArg rootfs hostname workdir -- argv...
	if len(os.Args) < 7 {
		fatalf("ccrun child: malformed re-exec arguments")
	}
	rootfs := os.Args[2]
	hostname := os.Args[3]
	workdir := os.Args[4]
	argv := os.Args[6:]

	if err := syscall.Sethostname([]byte(hostname)); err != nil {
		fatalf("ccrun child: sethostname: %v", err)
	}

	if err := pivotInto(rootfs); err != nil {
		fatalf("ccrun child: %v", err)
	}

	if err := mountProc(); err != nil {
		fatalf("ccrun child: %v", err)
	}

	if workdir == "" {
		workdir = "/"
	}
	if err := syscall.Chdir(workdir); err != nil {
		_ = syscall.Chdir("/")
	}

	bin, err := resolveBinary(argv[0])
	if err != nil {
		fatalf("ccrun: exec %s: %v", argv[0], err)
	}
	if err := syscall.Exec(bin, argv, os.Environ()); err != nil {
		fatalf("ccrun: exec %s: %v", bin, err)
	}
}

// pivotInto makes rootfs the root directory, using pivot_root where allowed
// and chroot otherwise.
func pivotInto(rootfs string) error {
	// Keep our mounts from propagating back to the host.
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("make mounts private: %w", err)
	}
	// pivot_root requires the new root to be a mount point; bind-mount it onto
	// itself to satisfy that.
	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind rootfs: %w", err)
	}

	putOld := filepath.Join(rootfs, ".pivot_old")
	if err := os.MkdirAll(putOld, 0o700); err != nil {
		return fmt.Errorf("create pivot_old: %w", err)
	}

	if err := syscall.PivotRoot(rootfs, putOld); err != nil {
		if cerr := syscall.Chroot(rootfs); cerr != nil {
			return fmt.Errorf("pivot_root (%v) and chroot (%v) both failed", err, cerr)
		}
		return syscall.Chdir("/")
	}

	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir to new root: %w", err)
	}
	if err := syscall.Unmount("/.pivot_old", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old root: %w", err)
	}
	return os.RemoveAll("/.pivot_old")
}

// mountProc mounts a procfs that only shows the container's processes.
func mountProc() error {
	if err := os.MkdirAll("/proc", 0o555); err != nil {
		return fmt.Errorf("create /proc: %w", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}
	return nil
}

// resolveBinary looks name up in the container's PATH unless it is already a
// path.
func resolveBinary(name string) (string, error) {
	if filepath.IsAbs(name) || filepath.Base(name) != name {
		return name, nil
	}
	return exec.LookPath(name)
}

// applyCgroup puts pid in a new cgroup v2 group with the given limits and
// returns a func that removes the group. Failures are only warnings.
func applyCgroup(pid int, memory, cpus string) func() {
	if memory == "" && cpus == "" {
		return func() {}
	}
	const base = "/sys/fs/cgroup"
	dir := filepath.Join(base, "ccrun-"+strconv.Itoa(pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		warnf("cgroup: create %s: %v", dir, err)
		return func() {}
	}

	if memory != "" {
		if b, err := ParseMemory(memory); err != nil {
			warnf("cgroup: %v", err)
		} else if err := os.WriteFile(filepath.Join(dir, "memory.max"), []byte(strconv.FormatInt(b, 10)), 0o644); err != nil {
			warnf("cgroup: set memory.max: %v", err)
		}
	}
	if cpus != "" {
		if q, err := ParseCPUQuota(cpus, DefaultCPUPeriod); err != nil {
			warnf("cgroup: %v", err)
		} else {
			line := fmt.Sprintf("%d %d", q, DefaultCPUPeriod)
			if err := os.WriteFile(filepath.Join(dir, "cpu.max"), []byte(line), 0o644); err != nil {
				warnf("cgroup: set cpu.max: %v", err)
			}
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0o644); err != nil {
		warnf("cgroup: add pid %d: %v", pid, err)
	}

	return func() { _ = os.Remove(dir) }
}

func warnf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ccrun: warning: "+format+"\n", a...)
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(127)
}
