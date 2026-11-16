package stack

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Forward port-forwards every access entry to 127.0.0.1 and keeps the
// forwards alive until ctx ends. kubectl port-forward exits when the pod
// behind it restarts, so each forward is restarted rather than left dead.
func (e *Engine) Forward(ctx context.Context) error {
	s := e.stack
	if len(s.Access) == 0 {
		return fmt.Errorf("stack %s lists nothing to access", s.Name)
	}
	if _, ok := readFileIfExists(e.kubeconfigPath()); !ok {
		return fmt.Errorf("no kubeconfig for %s — run `mozza stack up %s` first", s.Name, s.Name)
	}

	var wg sync.WaitGroup
	for _, a := range s.Access {
		wg.Add(1)
		go func(a Access) {
			defer wg.Done()
			e.keepForwarding(ctx, a)
		}(a)
	}
	for _, a := range s.Access {
		if a.Health == "" {
			continue
		}
		go e.reportHealth(ctx, a)
	}

	_, _ = fmt.Fprintf(e.out, "Forwarding (Ctrl-C to stop):\n")
	for _, a := range s.Access {
		_, _ = fmt.Fprintf(e.out, "  %-12s %s\n", a.Name, accessURL(a))
	}
	wg.Wait()
	return nil
}

func (e *Engine) keepForwarding(ctx context.Context, a Access) {
	pf := e.kubectl("port-forward", "--address", "127.0.0.1", "-n", a.Namespace,
		"svc/"+a.Service, fmt.Sprintf("%d:%d", localPort(a), a.Port))
	for ctx.Err() == nil {
		// Forward chatter ("Handling connection for ...") is noise here.
		err := e.runner.Run(ctx, pf, io.Discard)
		if ctx.Err() != nil {
			return
		}
		_, _ = fmt.Fprintf(e.out, "    %s: forward ended (%v), retrying\n", a.Name, err)
		if sleep(ctx, 2*time.Second) != nil {
			return
		}
	}
}

func (e *Engine) reportHealth(ctx context.Context, a Access) {
	url := "http://127.0.0.1:" + strconv.Itoa(localPort(a)) + a.Health
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(2 * time.Minute)
	for ctx.Err() == nil && time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				_, _ = fmt.Fprintf(e.out, "    %s: %s answered %d\n", a.Name, url, resp.StatusCode)
				return
			}
		}
		if sleep(ctx, 2*time.Second) != nil {
			return
		}
	}
	if ctx.Err() == nil {
		_, _ = fmt.Fprintf(e.out, "    %s: %s did not answer within 2m\n", a.Name, url)
	}
}
