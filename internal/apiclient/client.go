package apiclient

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arcgolabs/clientx"
	clientcodec "github.com/arcgolabs/clientx/codec"
	clientxhttp "github.com/arcgolabs/clientx/http"

	"github.com/daiyuang/orch/internal/api"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// DefaultBaseURL returns ORCH_SERVER if set, else local dev default matching orch-server HTTP.Addr (:17443).
func DefaultBaseURL() string {
	v := strings.TrimSpace(os.Getenv("ORCH_SERVER"))
	if v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://127.0.0.1:17443"
}

// Client is the orch control-plane HTTP facade (github.com/arcgolabs/clientx/http → resty for transport).
// Request/response JSON uses clientx [clientcodec.JSON] (implements [clientcodec.Codec]); TCP/UDP use the same codecs via DialCodec. clientx/http leaves bodies to callers/resty.
type Client struct {
	hc clientxhttp.Client
}

// New builds a client with base URL and optional bearer token (Authorization header).
func New(baseURL, bearerToken string) (*Client, error) {
	var opts []clientxhttp.Option
	tok := strings.TrimSpace(bearerToken)
	if tok != "" {
		opts = append(opts, clientxhttp.WithHeader("Authorization", "Bearer "+tok))
	}
	hc, err := clientxhttp.New(clientxhttp.Config{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Timeout: 60 * time.Second,
		Retry: clientx.RetryConfig{
			Enabled:    true,
			MaxRetries: 2,
			WaitMin:    200 * time.Millisecond,
			WaitMax:    2 * time.Second,
		},
	}, opts...)
	if err != nil {
		return nil, oopsx.B("cli", "apiclient").Wrapf(err, "create clientx http client")
	}
	return &Client{hc: hc}, nil
}

// Close releases idle connections held by the underlying transport.
func (c *Client) Close() error {
	if c == nil || c.hc == nil {
		return nil
	}
	if err := c.hc.Close(); err != nil {
		return oopsx.B("cli", "apiclient").Wrapf(err, "close http client")
	}
	return nil
}

// Health calls GET /api/health.
func (c *Client) Health(ctx context.Context) (*api.HealthOutput, error) {
	var out api.HealthOutput
	if err := c.get(ctx, api.PathHealth, &out.Body); err != nil {
		return nil, err
	}
	return &out, nil
}

// Hostinfo calls GET /api/v1/hostinfo.
func (c *Client) Hostinfo(ctx context.Context) (*api.HostinfoOutput, error) {
	var out api.HostinfoOutput
	if err := c.get(ctx, api.PathV1Hostinfo, &out.Body); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListWorkloads calls GET /api/v1/workloads.
func (c *Client) ListWorkloads(ctx context.Context) (*api.ListWorkloadsOutput, error) {
	var out api.ListWorkloadsOutput
	if err := c.get(ctx, api.PathV1Workloads, &out.Body); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAssignments calls GET /api/v1/assignments.
func (c *Client) ListAssignments(ctx context.Context) (*api.ListAssignmentsOutput, error) {
	var out api.ListAssignmentsOutput
	if err := c.get(ctx, api.PathV1Assignments, &out.Body); err != nil {
		return nil, err
	}
	return &out, nil
}

// Deploy calls POST /api/v1/deploy with a deploy DSL document.
func (c *Client) Deploy(ctx context.Context, app *deployv1.App) (*api.DeployOutput, error) {
	if app == nil {
		return nil, oopsx.B("cli", "apiclient").Errorf("nil app")
	}
	in := api.DeployInput{Body: *app}
	var out api.DeployOutput
	if err := c.post(ctx, api.PathV1Deploy, &in.Body, &out.Body); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeploySource calls POST /api/v1/deploy/source with manifest source; virtualPath should end with .orch or .yaml/.yml (or JSON-shaped YAML).
func (c *Client) DeploySource(ctx context.Context, virtualPath, source string) (*api.DeployOutput, error) {
	if virtualPath == "" {
		return nil, oopsx.B("cli", "apiclient").Errorf("empty virtualPath")
	}
	var in api.DeploySourceInput
	in.Body.VirtualPath = virtualPath
	in.Body.Source = source
	var out api.DeployOutput
	if err := c.post(ctx, api.PathV1DeploySource, &in.Body, &out.Body); err != nil {
		return nil, err
	}
	return &out, nil
}

// OrchVPNBootstrap calls GET /api/v1/orch-vpn/bootstrap.
func (c *Client) OrchVPNBootstrap(ctx context.Context) (*api.OrchVPNBootstrapOutput, error) {
	var out api.OrchVPNBootstrapOutput
	if err := c.get(ctx, api.PathV1OrchVPNBootstrap, &out.Body); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	if c == nil || c.hc == nil {
		return oopsx.B("cli", "apiclient").Errorf("nil client")
	}
	resp, err := c.hc.Execute(ctx, c.hc.R(), http.MethodGet, path)
	if err != nil {
		return oopsx.B("cli", "apiclient").Wrapf(err, "GET %s", path)
	}
	if !resp.IsSuccess() {
		msg := strings.TrimSpace(string(resp.Bytes()))
		return oopsx.B("cli", "apiclient").Errorf("GET %s: %s: %s", path, resp.Status(), msg)
	}
	if err := clientcodec.JSON.Unmarshal(resp.Bytes(), out); err != nil {
		return oopsx.B("cli", "apiclient").Wrapf(err, "GET %s response", path)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	if c == nil || c.hc == nil {
		return oopsx.B("cli", "apiclient").Errorf("nil client")
	}
	raw, err := clientcodec.JSON.Marshal(body)
	if err != nil {
		return oopsx.B("cli", "apiclient").Wrapf(err, "POST %s body", path)
	}
	req := c.hc.R().
		SetHeader("Content-Type", "application/json").
		SetBody(raw)
	resp, err := c.hc.Execute(ctx, req, http.MethodPost, path)
	if err != nil {
		return oopsx.B("cli", "apiclient").Wrapf(err, "POST %s", path)
	}
	if !resp.IsSuccess() {
		msg := strings.TrimSpace(string(resp.Bytes()))
		return oopsx.B("cli", "apiclient").Errorf("POST %s: %s: %s", path, resp.Status(), msg)
	}
	if err := clientcodec.JSON.Unmarshal(resp.Bytes(), out); err != nil {
		return oopsx.B("cli", "apiclient").Wrapf(err, "POST %s response", path)
	}
	return nil
}
