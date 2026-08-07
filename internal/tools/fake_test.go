package tools

import "context"

// fakeRunner is a scripted Runner for tools tests: it returns the configured
// output/err verbatim and records the last invocation as binary + args.
type fakeRunner struct {
	output []byte
	err    error
	args   []string
}

func (f *fakeRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	f.args = append([]string{binary}, args...)
	return f.output, f.err
}
