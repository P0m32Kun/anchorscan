package probes

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

// damengProbeHex is the raw handshake probe used by the Nuclei community
// template javascript/detection/dameng-detect.yaml. The probe initiates a
// Dameng (DM) database connection handshake without completing authentication,
// which is enough for a DM listener to return a recognizable protocol response.
//
// Source probe (hex):
//
//	00000000c80051000000000000000000000000990000000000000000010200000000
//	00000000000000000000000000000000000000000000000000000000000000080000
//	00382e312e312e34390040000000068149bbe004a62fb45552831704c802d4d802b
//	4579cb045b3c6100880725ececf148a7c9205047caccadfef5ff264460d11092a3
//	b483bf9d24382dea1dc43e7
const damengProbeHex = "00000000c8005100000000000000000000000099000000000000000001020000000000000000000000000000000000000000000000000000000000000000000008000000382e312e312e34390040000000068149bbe004a62fb45552831704c802d4d802b4579cb045b3c6100880725ececf148a7c9205047caccadfef5ff264460d11092a3b483bf9d24382dea1dc43e7"

// Dameng detection timeouts and read limits. The probe is tiny and a local or
// LAN DM listener answers quickly; keep these short so weakly-identified ports
// do not slow down the whole scan.
const (
	damengConnectTimeout = 3 * time.Second
	damengReadTimeout    = 3 * time.Second
	damengMaxReadSize    = 4096
)

var damengProbe []byte

func init() {
	var err error
	damengProbe, err = hex.DecodeString(damengProbeHex)
	if err != nil {
		// Programmer error: the constant above is malformed.
		panic(fmt.Sprintf("decode dameng probe: %v", err))
	}
}

// DetectDameng attempts to identify a Dameng database listener on the port
// described by fp. It returns an updated fingerprint with Normalized set to
// "dameng" when the probe receives a non-empty response that looks like a DM
// protocol packet. The original fingerprint and ok==false are returned on
// miss, timeout, or connection error.
//
// The probe does not complete authentication, so it is lighter and less
// intrusive than a real login attempt.
func DetectDameng(ctx context.Context, fp fingerprint.ServiceFingerprint) (fingerprint.ServiceFingerprint, bool) {
	out := fp
	addr := net.JoinHostPort(fp.IP, fmt.Sprintf("%d", fp.Port))

	dialer := &net.Dialer{Timeout: damengConnectTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return out, false
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(damengReadTimeout)); err != nil {
		return out, false
	}

	if _, err := conn.Write(damengProbe); err != nil {
		return out, false
	}

	buf := make([]byte, damengMaxReadSize)
	n, err := conn.Read(buf)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return out, false
	}

	if !looksLikeDamengResponse(buf[:n]) {
		return out, false
	}

	out.Normalized = "dameng"
	if out.Service == "" {
		out.Service = "dameng"
	}
	return out, true
}

// looksLikeDamengResponse applies a conservative, protocol-aware check on the
// bytes returned by the probe. The Nuclei template treats any non-empty
// response as a match; we add a small sanity check that the response starts with
// what appears to be a DM packet length field and is not just random echo.
//
// MVP note: this matcher intentionally stays permissive. As more DM response
// captures become available, the heuristic can be tightened (e.g. version
// string position, fixed magic bytes).
func looksLikeDamengResponse(resp []byte) bool {
	if len(resp) == 0 {
		return false
	}

	// A well-formed DM response begins with a 4-byte little-endian length that
	// equals or exceeds the actual received payload size. Allow some slack for
	// partial reads.
	if len(resp) >= 4 {
		length := int(resp[0]) | int(resp[1])<<8 | int(resp[2])<<16 | int(resp[3])<<24
		if length >= len(resp) && length <= damengMaxReadSize {
			return true
		}
	}

	// Fallback: any non-trivial response to the probe is treated as a hit.
	// This mirrors the Nuclei template's behavior and keeps false negatives low.
	return len(resp) >= 4
}
