package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLError struct {
	Message string `json:"message"`
}

type GraphQLResponse[T any] struct {
	Data   T              `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

func graphqlEndpoint(base *url.URL) (*url.URL, error) {
	if base == nil {
		return nil, fmt.Errorf("graphql: base url is nil")
	}

	u := *base
	u.RawQuery = ""
	u.Fragment = ""

	// GitHub.com REST base: https://api.github.com/
	// GitHub.com GraphQL:   https://api.github.com/graphql
	//
	// GHES REST base is typically: https://<host>/api/v3/
	// GHES GraphQL:               https://<host>/api/graphql
	path := strings.TrimSuffix(u.Path, "/")
	if strings.HasSuffix(path, "/api/v3") {
		u.Path = "/api/graphql"
		return &u, nil
	}

	// Default to host-root /graphql.
	u.Path = "/graphql"
	return &u, nil
}

// DoGraphQL executes a GraphQL POST against the GitHub API using the same underlying
// transport configuration as the REST client (auth, verbose logging, etc.).
//
// Callers are expected to handle request-budget accounting (Acquire/UpdateFromResponse)
// at the fetcher layer.
func DoGraphQL[T any](ctx context.Context, c *Client, req GraphQLRequest) (GraphQLResponse[T], *http.Response, error) {
	if ctx == nil {
		var zero GraphQLResponse[T]
		return zero, nil, fmt.Errorf("graphql: ctx is nil")
	}
	if c == nil || c.Client == nil {
		var zero GraphQLResponse[T]
		return zero, nil, fmt.Errorf("graphql: client is nil")
	}
	if c.HTTP == nil {
		var zero GraphQLResponse[T]
		return zero, nil, fmt.Errorf("graphql: http client is nil")
	}

	endpoint, err := graphqlEndpoint(c.Client.BaseURL)
	if err != nil {
		var zero GraphQLResponse[T]
		return zero, nil, err
	}

	body, err := json.Marshal(req)
	if err != nil {
		var zero GraphQLResponse[T]
		return zero, nil, fmt.Errorf("graphql: marshal request: %w", err)
	}

	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		var zero GraphQLResponse[T]
		return zero, nil, fmt.Errorf("graphql: build request: %w", err)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "application/json")

	hresp, err := c.HTTP.Do(hreq)
	if err != nil {
		var zero GraphQLResponse[T]
		return zero, nil, fmt.Errorf("graphql: do request: %w", err)
	}

	if hresp.StatusCode < 200 || hresp.StatusCode >= 300 {
		_ = hresp.Body.Close()
		var zero GraphQLResponse[T]
		return zero, hresp, fmt.Errorf("graphql: http %d", hresp.StatusCode)
	}

	var out GraphQLResponse[T]
	dec := json.NewDecoder(hresp.Body)
	if err := dec.Decode(&out); err != nil {
		_ = hresp.Body.Close()
		var zero GraphQLResponse[T]
		return zero, hresp, fmt.Errorf("graphql: decode response: %w", err)
	}
	_ = hresp.Body.Close()

	if len(out.Errors) > 0 {
		var zero GraphQLResponse[T]
		return zero, hresp, fmt.Errorf("graphql: %s", out.Errors[0].Message)
	}

	return out, hresp, nil
}
