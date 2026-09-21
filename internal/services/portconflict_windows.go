//go:build windows

package services

import (
	"errors"
	"fmt"
	"syscall"
)

// portConflictIsTransient reports whether a failed bind may succeed shortly.
//
// Windows hands out outbound connections from a dynamic range that starts at
// 1024 (netsh int ipv4 show dynamicport tcp). A CONNECTED socket owns its
// local port outright, and binding a LISTENER on that same address:port then
// fails with WSAEACCES (10013) — "an attempt was made to access a socket in a
// way forbidden by its access permissions" — not with the WSAEADDRINUSE
// (10048) you would get from a real listener. Measured directly: errno 10013,
// and neither errors.Is(err, fs.ErrPermission) nor syscall.EACCES matches it,
// so the comparison has to be against WSAEACCES itself.
//
// This is exactly how mailpit's SMTP port 1025 breaks: it sits inside that
// dynamic range, so a stray loopback connection from any other application
// (Herd's mysqld, a browser, anything) can hold it while nothing at all is
// listening there. The condition is transient, so a short retry is worth it.
func portConflictIsTransient(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == syscall.WSAEACCES
}

// foreignPortHint replaces the misleading "is another instance running?"
// message when the port is not actually taken by a listener. Without this the
// user goes hunting for a phantom process that does not exist.
func foreignPortHint(port int, err error) string {
	if !portConflictIsTransient(err) {
		return ""
	}
	return fmt.Sprintf(
		"Windows refused the bind but nothing is listening on %d — that port is "+
			"inside the OS dynamic range (1024+) and a transient outbound "+
			"connection holds it. Retry in a moment; check with: netstat -ano | findstr :%d",
		port, port)
}
