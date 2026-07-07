package status

import (
	"net"
	"testing"
	"time"
)

func TestProbeOpenPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	addr := ln.Addr().(*net.TCPAddr)
	if !Probe(addr.IP.String(), addr.Port, time.Second) {
		t.Fatal("expected Probe to succeed against an open port")
	}
}

func TestProbeClosedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	_ = ln.Close() // free the port so the dial gets refused

	if Probe(addr.IP.String(), addr.Port, time.Second) {
		t.Fatal("expected Probe to fail against a closed port")
	}
}

func TestProbeUnroutableAddressTimesOut(t *testing.T) {
	// TEST-NET-1 (RFC 5737), guaranteed non-routable — exercises the
	// timeout path rather than an immediate refusal.
	start := time.Now()
	if Probe("192.0.2.1", 22, 200*time.Millisecond) {
		t.Fatal("expected Probe to fail against an unroutable address")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("expected Probe to respect its timeout, took %v", elapsed)
	}
}
