//go:build windows

// conpty_windows.go — a real pseudo-terminal for Windows via ConPTY,
// replacing the old pipe fallback so colors, resize and interactive
// programs work in the embedded terminal.
//
// The ConPTY plumbing is adapted from aymanbagabas/go-pty v0.2.3
// (pty_windows.go / cmd_windows.go), MIT licensed — trimmed to what the
// terminal package needs: one console, one child process.
package terminal

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// UpdateProcThreadAttribute is called directly rather than through
// x/sys/windows' ProcThreadAttributeListContainer.Update. That wrapper is
// built for attributes whose lpValue is a pointer to data (a HANDLE list,
// say), but PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE wants lpValue to BE the
// HPCON value — Microsoft's EchoCon sample passes hPC itself, and so does
// go-pty. Feeding it &hpc makes CreateProcess SUCCEED while leaving the
// child attached to something that is not our pseudo console: the shell
// runs and exits 0, yet not one byte ever reaches outPipe (a silently dead
// terminal). Calling the Win32 entry point directly also keeps lpValue a
// plain uintptr syscall argument, so there is no uintptr→unsafe.Pointer
// conversion for go vet's unsafeptr check to flag.
var (
	modkernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	procUpdateProcThreadAttribute = modkernel32.NewProc("UpdateProcThreadAttribute")
)

// conPty is one Windows pseudo console plus its two plumbing pipes:
// inPipe (we write → console input) and outPipe (console output → we read).
type conPty struct {
	handle  windows.Handle
	inPipe  *os.File
	outPipe *os.File
	proc    windows.Handle // child process (terminated on Close)
}

// newConPTY allocates the pseudo console (80x24; resized once xterm fits).
func newConPTY() (*conPty, error) {
	ptyIn, inOurs, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("conpty input pipe: %w", err)
	}
	outOurs, ptyOut, err := os.Pipe()
	if err != nil {
		_ = ptyIn.Close()
		_ = inOurs.Close()
		return nil, fmt.Errorf("conpty output pipe: %w", err)
	}

	var hpc windows.Handle
	coord := windows.Coord{X: 80, Y: 24}
	if err := windows.CreatePseudoConsole(coord, windows.Handle(ptyIn.Fd()), windows.Handle(ptyOut.Fd()), 0, &hpc); err != nil {
		_ = ptyIn.Close()
		_ = inOurs.Close()
		_ = outOurs.Close()
		_ = ptyOut.Close()
		return nil, fmt.Errorf("CreatePseudoConsole: %w", err)
	}
	// Child-side ends are owned by the console now.
	_ = ptyOut.Close()
	_ = ptyIn.Close()

	return &conPty{handle: hpc, inPipe: inOurs, outPipe: outOurs}, nil
}

// startProcess launches exe with args attached to the pseudo console.
func (c *conPty) startProcess(exe string, args []string, dir string, env []string) error {
	pathPtr, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return fmt.Errorf("exe path: %w", err)
	}
	cmdline, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{exe}, args...)))
	if err != nil {
		return fmt.Errorf("command line: %w", err)
	}
	var dirPtr *uint16
	if dir != "" {
		if dirPtr, err = windows.UTF16PtrFromString(dir); err != nil {
			return fmt.Errorf("work dir: %w", err)
		}
	}

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return fmt.Errorf("attribute list: %w", err)
	}
	defer attrs.Delete()

	// Attach the child to our pseudo console: without this CreateProcess is
	// never told about hpc, so Windows allocates a fresh console WINDOW for
	// the child (PowerShell pops up externally) and the inPipe/outPipe stay
	// unwired — i.e. the embedded terminal is dead and a real console window
	// appears on the desktop.
	//
	// lpValue (the 4th argument) must be the HPCON VALUE, not a pointer to
	// it: pass c.handle straight through as the raw uintptr slot.
	r1, _, callErr := procUpdateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(attrs.List())),
		0,
		uintptr(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE),
		uintptr(c.handle),
		unsafe.Sizeof(c.handle),
		0,
		0,
	)
	if r1 == 0 {
		return fmt.Errorf("pseudoconsole attribute: %w", callErr)
	}

	siEx := &windows.StartupInfoEx{}
	// STARTF_USESTDHANDLES is what makes Windows route the child's std handles
	// through the pseudo console instead of handing it the parent's console.
	// Without it CreateProcess still succeeds and the shell runs happily — on
	// the SERVER's console, printing its prompt into the sidecar's own output
	// while the dashboard terminal stays empty. (go-pty, whose plumbing this
	// follows, sets it too.)
	siEx.Flags = windows.STARTF_USESTDHANDLES
	siEx.Cb = uint32(unsafe.Sizeof(*siEx))
	siEx.ProcThreadAttributeList = attrs.List()
	pi := &windows.ProcessInformation{}

	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT) | windows.EXTENDED_STARTUPINFO_PRESENT
	envBlock, err := envBlockUTF16(env)
	if err != nil {
		return err
	}
	// Inheritable process/thread handles, matching go-pty; the child's own std
	// handles come from the pseudo console via the attribute list above.
	var zeroSec windows.SecurityAttributes
	pSec := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(zeroSec)), InheritHandle: 1}
	tSec := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(zeroSec)), InheritHandle: 1}
	if err := windows.CreateProcess(
		pathPtr,
		cmdline,
		pSec,
		tSec,
		false,
		flags,
		&envBlock[0],
		dirPtr,
		&siEx.StartupInfo,
		pi,
	); err != nil {
		return fmt.Errorf("CreateProcess: %w", err)
	}
	c.proc = pi.Process
	_ = windows.CloseHandle(pi.Thread)
	return nil
}

// resize updates the console size (cols x rows).
func (c *conPty) resize(cols, rows int) error {
	if c.handle == 0 {
		return fmt.Errorf("pseudo console closed")
	}
	if err := windows.ResizePseudoConsole(c.handle, windows.Coord{X: int16(cols), Y: int16(rows)}); err != nil {
		return fmt.Errorf("ResizePseudoConsole: %w", err)
	}
	return nil
}

// close terminates the shell and releases the console.
func (c *conPty) close() {
	if c.proc != 0 {
		_ = windows.TerminateProcess(c.proc, 1)
		_, _ = windows.WaitForSingleObject(c.proc, 5000)
		_ = windows.CloseHandle(c.proc)
		c.proc = 0
	}
	if c.handle != 0 {
		windows.ClosePseudoConsole(c.handle)
		c.handle = 0
	}
	_ = c.inPipe.Close()
	_ = c.outPipe.Close()
}

// envBlockUTF16 builds a NUL-separated UTF-16 environment block as expected
// by CreateProcess ("k=v\0k=v\0…\0\0").
func envBlockUTF16(env []string) ([]uint16, error) {
	block := strings.Join(env, "\x00") + "\x00\x00"
	runes := utf16.Encode([]rune(block))
	if len(runes) > 0x7fff {
		return nil, fmt.Errorf("environment block too large")
	}
	return runes, nil
}
