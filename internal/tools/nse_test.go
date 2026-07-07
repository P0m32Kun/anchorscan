package tools

import (
	"context"
	"reflect"
	"testing"
)

func TestRunNSEBuildsCommandAndParsesScripts(t *testing.T) {
	runner := &fakeRunner{
		output: []byte(`<nmaprun><host><ports><port protocol="tcp" portid="6379"><script id="redis-info" output="Redis version 7.0"/></port></ports></host></nmaprun>`),
	}

	got, err := RunNSE(context.Background(), runner, "/opt/nmap", "192.168.1.10", 6379, []string{"redis-info"}, nil)
	if err != nil {
		t.Fatalf("RunNSE returned error: %v", err)
	}

	wantArgs := []string{"/opt/nmap", "-p", "6379", "--script", "redis-info", "192.168.1.10", "-oX", "-"}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("args mismatch: got %#v want %#v", runner.args, wantArgs)
	}
	if len(got) != 1 || got[0].ID != "redis-info" || got[0].Output != "Redis version 7.0" {
		t.Fatalf("unexpected results: %#v", got)
	}
}
