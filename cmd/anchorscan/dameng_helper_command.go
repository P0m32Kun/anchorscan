package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/P0m32Kun/anchorscan/internal/tools"
)

func runInternalDamengCheck(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("internal-dameng-check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	host := fs.String("host", "", "target host")
	port := fs.Int("port", 0, "target port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *host == "" || *port < 1 || *port > 65535 {
		return fmt.Errorf("invalid dameng target")
	}
	result, err := tools.RunDamengDefaultPassword(context.Background(), tools.DefaultDamengChecker, *host, *port)
	if encodeErr := json.NewEncoder(stdout).Encode(result); encodeErr != nil {
		return encodeErr
	}
	return err
}
