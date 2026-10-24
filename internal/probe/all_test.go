package probe

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/jasonli0226/climan/internal/platform"
)

func TestAllParallel(t *testing.T) {
	paths := map[string]string{"a": "/a", "b": "/b", "c": "/c", "d": "/d"}
	fn := func(_ context.Context, _ platform.Env, path string) Info {
		time.Sleep(300 * time.Millisecond)
		return Info{Version: "v" + path}
	}
	start := time.Now()
	got := All(context.Background(), platform.Env{}, paths, fn, &bytes.Buffer{})
	if took := time.Since(start); took > 900*time.Millisecond {
		t.Fatalf("4 probes of 300ms took %v, want parallel", took)
	}
	if len(got) != 4 || got["c"].Version != "v/c" {
		t.Fatalf("All = %v", got)
	}
}

func TestAllEmptyPrintsNothing(t *testing.T) {
	var out bytes.Buffer
	got := All(context.Background(), platform.Env{}, nil, nil, &out)
	if len(got) != 0 || out.Len() != 0 {
		t.Fatalf("All(empty) = %v, output %q", got, out.String())
	}
}

func TestAllProgressLine(t *testing.T) {
	var out bytes.Buffer
	fn := func(context.Context, platform.Env, string) Info { return Info{} }
	All(context.Background(), platform.Env{}, map[string]string{"a": "/a"}, fn, &out)
	if out.String() != "probing 1 new tool for version and help...\n" {
		t.Fatalf("output = %q", out.String())
	}
}
