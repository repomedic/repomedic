package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

type Client struct {
	Client *github.Client
	HTTP   *http.Client
}

type options struct {
	verbose bool
	// writer controls where verbose HTTP logs are written (typically stderr) so
	// structured output on stdout (e.g. NDJSON) stays clean and tests can capture logs.
	writer io.Writer
}

type Option func(*options)

func WithVerbose(enabled bool, writer io.Writer) Option {
	return func(o *options) {
		o.verbose = enabled
		o.writer = writer
	}
}

// loggingRoundTripper wraps an underlying transport and emits one line per
// request and response (including latency) when verbose logging is enabled.
type loggingRoundTripper struct {
	base http.RoundTripper
	w    io.Writer
}

func (t *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	if t.w != nil {
		_, _ = fmt.Fprintf(t.w, "[verbose] github api: %s %s\n", req.Method, req.URL.String())
	}
	resp, err := t.base.RoundTrip(req)
	dur := time.Since(start)
	if t.w != nil {
		if err != nil {
			_, _ = fmt.Fprintf(t.w, "[verbose] github api: error after %s: %v\n", dur.Truncate(time.Millisecond), err)
		} else {
			_, _ = fmt.Fprintf(t.w, "[verbose] github api: %d %s (%s)\n", resp.StatusCode, http.StatusText(resp.StatusCode), dur.Truncate(time.Millisecond))
		}
	}
	return resp, err
}

func NewClient(ctx context.Context, token string, opts ...Option) (*Client, error) {
	if ctx == nil {
		return nil, fmt.Errorf("github client: ctx is nil")
	}

	o := &options{}
	for _, apply := range opts {
		if apply != nil {
			apply(o)
		}
	}
	if o.verbose && o.writer == nil {
		o.writer = os.Stderr
	}

	transport := http.DefaultTransport
	if o.verbose {
		transport = &loggingRoundTripper{base: transport, w: o.writer}
	}
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		transport = &oauth2.Transport{Source: ts, Base: transport}
	}
	// Always provide an http.Client so verbose logging works even without a token.
	tc := &http.Client{Transport: transport}

	return &Client{
		Client: github.NewClient(tc),
		HTTP:   tc,
	}, nil
}
