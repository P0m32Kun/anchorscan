package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	dm "gitee.com/chunanyong/dm"
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
	Verdict  DamengVerdict `json:"verdict"`
	Username string        `json:"username,omitempty"`
	Password string        `json:"password,omitempty"`
	Output   string        `json:"output"`
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

func callDamengChecker(ctx context.Context, checker DamengAuthChecker, host string, port int, username, password string) (ok bool, detail string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &damengDriverPanicError{value: recovered}
		}
	}()
	return checker.Check(ctx, host, port, username, password)
}

// RunDamengDefaultPassword checks the known historical SYSDBA weak passwords.
// Authentication success is treated as vulnerable; all candidates being
// rejected means the password has been changed; network or driver errors
// remain unknown so they are not misclassified as a finding.
func RunDamengDefaultPassword(ctx context.Context, checker DamengAuthChecker, ip string, port int) (DamengResult, error) {
	if checker == nil {
		checker = DefaultDamengChecker
	}

	for _, credential := range []struct {
		username string
		password string
	}{
		{username: "SYSDBA", password: "SYSDBA"},
		{username: "SYSDBA", password: "SYSDBA001"},
	} {
		ok, detail, err := callDamengChecker(ctx, checker, ip, port, credential.username, credential.password)
		if err != nil {
			var panicErr *damengDriverPanicError
			if errors.As(err, &panicErr) || errors.Is(err, context.DeadlineExceeded) {
				return DamengResult{Verdict: DamengUnknown, Output: err.Error()}, err
			}
			if IsDamengAuthError(err) {
				continue
			}
			return DamengResult{Verdict: DamengUnknown, Output: err.Error()}, err
		}
		if ok {
			if detail == "" {
				detail = fmt.Sprintf("authentication succeeded for %s/%s", credential.username, credential.password)
			}
			return DamengResult{Verdict: DamengVulnerable, Username: credential.username, Password: credential.password, Output: detail}, nil
		}
	}
	return DamengResult{Verdict: DamengSafe, Output: "authentication rejected"}, nil
}

func RunDamengHelper(ctx context.Context, runner Runner, executable, host string, port int) (DamengResult, error) {
	out, err := runner.Run(ctx, executable, []string{"internal-dameng-check", "--host", host, "--port", strconv.Itoa(port)})
	if err != nil {
		return DamengResult{Verdict: DamengUnknown, Output: string(out)}, withOutputError(err, out)
	}
	var result DamengResult
	if err := json.Unmarshal(out, &result); err != nil {
		return DamengResult{}, fmt.Errorf("parse dameng helper output: %w", err)
	}
	return result, nil
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
	var dmErr *dm.DmError
	if errors.As(err, &dmErr) && dmErr.ErrCode == -2501 {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "login") ||
		strings.Contains(msg, "authentication") ||
		strings.Contains(msg, "password") ||
		strings.Contains(msg, "权限") ||
		strings.Contains(msg, "口令") ||
		strings.Contains(msg, "密码")
}
