package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, binary string, args []string) ([]byte, error)
}

type ExecRunner struct{}

func NewExecRunner() Runner {
	return ExecRunner{}
}

func (ExecRunner) Run(ctx context.Context, binary string, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, binary, args...).CombinedOutput()
}

func withOutputError(err error, out []byte) error {
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, msg)
}
