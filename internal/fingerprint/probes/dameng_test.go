package probes

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestDetectDameng_Hit(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	// Fake DM listener: read the probe, then send a response that looks like a
	// DM packet (4-byte little-endian length >= payload).
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		if n == 0 {
			return
		}
		resp := []byte{0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		resp[0] = byte(len(resp))
		_, _ = conn.Write(resp)
	}()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port := mustPort(t, portStr)

	fp := fingerprint.ServiceFingerprint{IP: "127.0.0.1", Port: port}
	updated, ok := DetectDameng(context.Background(), fp)
	if !ok {
		t.Fatalf("expected dameng detection to succeed")
	}
	if updated.Normalized != "dameng" {
		t.Fatalf("expected Normalized=dameng, got %q", updated.Normalized)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("server goroutine did not finish")
	}
}

func TestDetectDameng_Miss(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		// Send back an empty response so the probe misses.
	}()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port := mustPort(t, portStr)

	fp := fingerprint.ServiceFingerprint{IP: "127.0.0.1", Port: port}
	updated, ok := DetectDameng(context.Background(), fp)
	if ok {
		t.Fatalf("expected no dameng detection")
	}
	if updated.Normalized != fp.Normalized {
		t.Fatalf("expected unchanged fingerprint")
	}
}

func TestDetectDameng_NoListener(t *testing.T) {
	fp := fingerprint.ServiceFingerprint{IP: "127.0.0.1", Port: 1}
	_, ok := DetectDameng(context.Background(), fp)
	if ok {
		t.Fatalf("expected no detection against port 1")
	}
}

func TestDetectDameng_ProbeConstant(t *testing.T) {
	if _, err := hex.DecodeString(damengProbeHex); err != nil {
		t.Fatalf("damengProbeHex is invalid: %v", err)
	}
}

func mustPort(t *testing.T, s string) int {
	t.Helper()
	var port int
	if _, err := net.LookupPort("tcp", s); err == nil {
		// net.LookupPort returns 0 for numeric strings on some platforms; fall back.
	}
	if _, err := fmt.Sscanf(s, "%d", &port); err != nil {
		t.Fatalf("invalid port %q: %v", s, err)
	}
	return port
}
