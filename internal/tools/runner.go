package tools

import (
	"context"
	"os/exec"
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
