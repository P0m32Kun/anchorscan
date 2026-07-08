package tools

import (
	"context"
	"reflect"
	"testing"
)

type aliveRunner struct {
	args []string
	out  []byte
}

func (r *aliveRunner) Run(_ context.Context, _ string, args []string) ([]byte, error) {
	r.args = append([]string(nil), args...)
	return r.out, nil
}

func TestCheckAliveBuildsNmapPingAndParsesUpHost(t *testing.T) {
	runner := &aliveRunner{out: []byte(`<nmaprun><host><status state="up"/></host></nmaprun>`)}

	alive, err := CheckAlive(context.Background(), runner, "nmap", "192.0.2.10", []string{"--min-rate", "50"})
	if err != nil {
		t.Fatal(err)
	}
	if !alive {
		t.Fatal("expected host to be alive")
	}

	want := []string{"-sn", "192.0.2.10", "-oX", "-", "--min-rate", "50"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("args = %#v, want %#v", runner.args, want)
	}
}

func TestCheckAliveReturnsFalseForDownHost(t *testing.T) {
	runner := &aliveRunner{out: []byte(`<nmaprun><host><status state="down"/></host></nmaprun>`)}

	alive, err := CheckAlive(context.Background(), runner, "nmap", "192.0.2.10", nil)
	if err != nil {
		t.Fatal(err)
	}
	if alive {
		t.Fatal("expected host to be down")
	}
}
