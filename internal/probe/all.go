package probe

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/jasonli0226/climan/internal/platform"
)

// maxParallel bounds how many probed tools run at once.
const maxParallel = 8

// Func probes one tool; Probe is the real implementation.
type Func func(ctx context.Context, env platform.Env, path string) Info

// All probes each path (keyed by tool) at most maxParallel at a time, after
// printing a progress line to out. An empty map prints nothing.
func All(ctx context.Context, env platform.Env, paths map[string]string, fn Func, out io.Writer) map[string]Info {
	infos := make(map[string]Info, len(paths))
	if len(paths) == 0 {
		return infos
	}
	fmt.Fprintf(out, "probing %d new %s for version and help...\n", len(paths), plural(len(paths), "tool"))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallel)
	for key, path := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			info := fn(ctx, env, path)
			mu.Lock()
			infos[key] = info
			mu.Unlock()
		}()
	}
	wg.Wait()
	return infos
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
