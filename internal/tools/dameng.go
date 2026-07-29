package tools

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	_ "gitee.com/chunanyong/dm"
)

// DamengVerdict is the outcome of a default-password check against a Dameng
// (DM) database listener. It mirrors the three-state shape used by rdpscan so
// that scan_target.go can treat the engine uniformly.
type DamengVerdict string

const (
	DamengVulnerable DamengVerdict = "vulnerable"
	DamengSafe       DamengVerdict = "safe"
	DamengUnknown    DamengVerdict = "unknown"
)

// DamengResult carries the classified verdict and any human-readable output.
type DamengResult struct {
	Verdict DamengVerdict
	Output  string
}

// DamengAuthChecker abstracts the actual login attempt so tests can inject
// success/failure/error without importing the real database driver.
type DamengAuthChecker interface {
	// Check attempts to authenticate with the supplied credentials and returns
	// true if the login succeeds. The returned string is optional detail text.
	Check(ctx context.Context, host string, port int, username, password string) (bool, string, error)
}

// damengDriverChecker uses the real Go DM driver.
type damengDriverChecker struct{}

func (c *damengDriverChecker) Check(ctx context.Context, host string, port int, username, password string) (bool, string, error) {
	dsn := fmt.Sprintf("dm://%s:%s@%s:%d", url.PathEscape(username), url.PathEscape(password), host, port)

	db, err := sql.Open("dm", dsn)
	if err != nil {
		return false, "", err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return false, "", err
	}
	return true, "", nil
}

// DefaultDamengChecker is the production implementation backed by the DM Go driver.
var DefaultDamengChecker DamengAuthChecker = &damengDriverChecker{}

type damengDriverPanicError struct {
	value any
}

func (e *damengDriverPanicError) Error() string {
	return fmt.Sprintf("dameng driver panic: %v", e.value)
}

func callDamengChecker(ctx context.Context, checker DamengAuthChecker, host string, port int) (ok bool, detail string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &damengDriverPanicError{value: recovered}
		}
	}()
	return checker.Check(ctx, host, port, "SYSDBA", "SYSDBA")
}

// RunDamengDefaultPassword checks whether a Dameng listener still uses the
// factory default credential SYSDBA/SYSDBA. Authentication success is treated
// as vulnerable; authentication failure means the password has been changed;
// network or driver errors remain unknown so they are not misclassified as a
// finding.
func RunDamengDefaultPassword(ctx context.Context, checker DamengAuthChecker, ip string, port int) (DamengResult, error) {
	if checker == nil {
		checker = DefaultDamengChecker
	}

	ok, detail, err := callDamengChecker(ctx, checker, ip, port)
	if err != nil {
		var panicErr *damengDriverPanicError
		if errors.As(err, &panicErr) || errors.Is(err, context.DeadlineExceeded) {
			return DamengResult{Verdict: DamengUnknown, Output: err.Error()}, err
		}
		// Distinguish an actual authentication rejection from a connection-level
		// problem. The DM driver returns errors containing authentication or
		// login keywords when credentials are rejected; everything else is
		// unknown (service not reachable, protocol mismatch, etc.).
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "login") ||
			strings.Contains(msg, "authentication") ||
			strings.Contains(msg, "password") ||
			strings.Contains(msg, "权限") ||
			strings.Contains(msg, "口令") {
			return DamengResult{Verdict: DamengSafe, Output: "authentication rejected: " + err.Error()}, nil
		}
		return DamengResult{Verdict: DamengUnknown, Output: err.Error()}, nil
	}

	if ok {
		return DamengResult{Verdict: DamengVulnerable, Output: detail}, nil
	}
	return DamengResult{Verdict: DamengSafe, Output: "authentication rejected"}, nil
}

// ParseDamengResult is a small helper for callers that already have a string
// verdict; it normalizes unexpected values to DamengUnknown.
func ParseDamengResult(s string) DamengVerdict {
	switch DamengVerdict(strings.ToLower(strings.TrimSpace(s))) {
	case DamengVulnerable:
		return DamengVulnerable
	case DamengSafe:
		return DamengSafe
	default:
		return DamengUnknown
	}
}

// IsDamengAuthError returns true if the error looks like a rejected credential
// rather than a transport/protocol failure. It is exported so tests and
// callers can use the same classification logic.
func IsDamengAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "login") ||
		strings.Contains(msg, "authentication") ||
		strings.Contains(msg, "password") ||
		strings.Contains(msg, "权限") ||
		strings.Contains(msg, "口令")
}
