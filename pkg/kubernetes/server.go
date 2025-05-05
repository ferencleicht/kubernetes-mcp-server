package kubernetes

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewServer(version string, opts ...server.ServerOption) *server.MCPServer {
	defaultOpts := []server.ServerOption{
		server.WithToolCapabilities(true),
		server.WithLogging(),
	}
	opts = append(defaultOpts, opts...)

	s := server.NewMCPServer(
		"kubernetes-mcp-server",
		version,
		opts...,
	)
	return s
}

func requiredParam[T comparable](r mcp.CallToolRequest, p string) (T, error) {
	var zero T

	if _, ok := r.Params.Arguments[p]; !ok {
		return zero, fmt.Errorf("missing required parameter: %s", p)
	}

	if _, ok := r.Params.Arguments[p].(T); !ok {
		return zero, fmt.Errorf("parameter %s is not of type %T", p, zero)
	}

	if r.Params.Arguments[p].(T) == zero {
		return zero, fmt.Errorf("missing required parameter: %s", p)
	}

	return r.Params.Arguments[p].(T), nil
}
