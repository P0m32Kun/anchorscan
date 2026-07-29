package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeDamengChecker struct {
	ok  bool
	out string
	err error
}

func (f *fakeDamengChecker) Check(ctx context.Context, host string, port int, username, password string) (bool, string, error) {
	return f.ok, f.out, f.err
}

func TestRunDamengDefaultPassword_Vulnerable(t *testing.T) {
	checker := &fakeDamengChecker{ok: true, out: "", err: nil}
	res, err := RunDamengDefaultPassword(context.Background(), checker, "127.0.0.1", 5236)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != DamengVulnerable {
		t.Fatalf("expected vulnerable, got %q", res.Verdict)
	}
}

func TestRunDamengDefaultPassword_Safe(t *testing.T) {
	checker := &fakeDamengChecker{ok: false, out: "", err: errors.New("login failed")}
	res, err := RunDamengDefaultPassword(context.Background(), checker, "127.0.0.1", 5236)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != DamengSafe {
		t.Fatalf("expected safe, got %q", res.Verdict)
	}
}

type panicDamengChecker struct{}

func (panicDamengChecker) Check(context.Context, string, int, string, string) (bool, string, error) {
	panic("driver index out of range")
}

func TestRunDamengDefaultPassword_RecoversCheckerPanic(t *testing.T) {
	res, err := RunDamengDefaultPassword(context.Background(), panicDamengChecker{}, "127.0.0.1", 5236)
	if err == nil || !strings.Contains(err.Error(), "dameng driver panic") {
		t.Fatalf("error = %v, want recovered driver panic", err)
	}
	if res.Verdict != DamengUnknown || !strings.Contains(res.Output, "driver index out of range") {
		t.Fatalf("result = %#v, want unknown result with panic detail", res)
	}
}

func TestRunDamengDefaultPassword_ReturnsDeadlineFailure(t *testing.T) {
	checker := &fakeDamengChecker{err: context.DeadlineExceeded}
	res, err := RunDamengDefaultPassword(context.Background(), checker, "127.0.0.1", 5236)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
	if res.Verdict != DamengUnknown || !strings.Contains(res.Output, "deadline exceeded") {
		t.Fatalf("result = %#v, want unknown result with deadline detail", res)
	}
}

func TestRunDamengDefaultPassword_UnknownNetworkError(t *testing.T) {
	checker := &fakeDamengChecker{ok: false, out: "", err: errors.New("connection refused")}
	res, err := RunDamengDefaultPassword(context.Background(), checker, "127.0.0.1", 5236)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != DamengUnknown {
		t.Fatalf("expected unknown for network error, got %q", res.Verdict)
	}
}

func TestRunDamengDefaultPassword_NilCheckerUsesDefault(t *testing.T) {
	// Nil checker should fall back to DefaultDamengChecker; on a closed port it
	// returns unknown without panicking.
	res, err := RunDamengDefaultPassword(context.Background(), nil, "127.0.0.1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != DamengUnknown {
		t.Fatalf("expected unknown against closed port, got %q", res.Verdict)
	}
}

func TestParseDamengResult(t *testing.T) {
	cases := []struct {
		in   string
		want DamengVerdict
	}{
		{"vulnerable", DamengVulnerable},
		{"VULNERABLE", DamengVulnerable},
		{"safe", DamengSafe},
		{"Safe ", DamengSafe},
		{"", DamengUnknown},
		{"other", DamengUnknown},
	}
	for _, c := range cases {
		if got := ParseDamengResult(c.in); got != c.want {
			t.Errorf("ParseDamengResult(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsDamengAuthError(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"login failed", true},
		{"Authentication error", true},
		{"invalid password", true},
		{"connection refused", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsDamengAuthError(errors.New(c.in)); got != c.want {
			t.Errorf("IsDamengAuthError(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
