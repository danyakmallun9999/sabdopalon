package proxy

import (
	"net"
	"sync/atomic"
	"testing"
)

// freeClosedPort returns a port number that nothing is listening on: bind it,
// note the number, let it go.
func freeClosedPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func listeningPort(t *testing.T) (int, net.Listener) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return l.Addr().(*net.TCPAddr).Port, l
}

// TestWorkerAlive pins the liveness probe the worker selection rests on.
func TestWorkerAlive(t *testing.T) {
	livePort, l := listeningPort(t)
	defer l.Close()
	if !workerAlive(livePort) {
		t.Errorf("port %d is listening but reported dead", livePort)
	}
	if workerAlive(freeClosedPort(t)) {
		t.Error("a port nothing listens on reported alive")
	}
}

// TestPickWorkerSkipsDeadWorkers: a `php -S` child can die without anyone
// noticing — memory pressure, a fatal error, or an app update that killed its
// parent without cleanup. Round-robin then handed that dead port one request in
// len(ports), which the browser shows as a blank page until a refresh happens to
// land on a live worker. Skipping dead ports is what turns that into a
// non-event.
func TestPickWorkerSkipsDeadWorkers(t *testing.T) {
	livePort, l := listeningPort(t)
	defer l.Close()
	deadPort := freeClosedPort(t)

	var rr atomic.Uint64
	ports := []int{deadPort, livePort}

	// Every call must land on the live worker, whichever one round-robin starts
	// from — the dead port is never chosen.
	for i := 0; i < 8; i++ {
		if got := pickWorker(ports, &rr); got != livePort {
			t.Fatalf("call %d: pickWorker = %d, want the live port %d", i, got, livePort)
		}
	}

	// A dead port in front of nothing else still returns a port, so the error the
	// user sees is the real connection failure rather than a silent hang.
	if got := pickWorker([]int{deadPort}, &rr); got != deadPort {
		t.Errorf("with no live worker pickWorker = %d, want %d", got, deadPort)
	}

	// An empty list is not a crash.
	if got := pickWorker(nil, &rr); got != 0 {
		t.Errorf("with no ports pickWorker = %d, want 0", got)
	}
}

// TestPickWorkerStillRotates pins that liveness did not flatten the round-robin:
// spreading concurrent requests is the whole reason several workers exist (the
// built-in server answers one request per process, so a single child looks
// frozen the moment a page fires an XHR while it is still rendering).
func TestPickWorkerStillRotates(t *testing.T) {
	p1, l1 := listeningPort(t)
	defer l1.Close()
	p2, l2 := listeningPort(t)
	defer l2.Close()

	var rr atomic.Uint64
	ports := []int{p1, p2}
	seen := map[int]int{}
	for i := 0; i < 6; i++ {
		seen[pickWorker(ports, &rr)]++
	}
	if seen[p1] != 3 || seen[p2] != 3 {
		t.Errorf("round-robin spread = %v, want 3 and 3", seen)
	}
}
