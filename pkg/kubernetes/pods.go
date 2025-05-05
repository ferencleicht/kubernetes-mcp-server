package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ListPods(getClient GetClientFn) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool(
		"list_pods",
		mcp.WithDescription("List all pods in a namespace"),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title: "List Pods",
			ReadOnlyHint: true,
		}),
		mcp.WithString("namespace",
			mcp.Required(),
			mcp.Description("Namespace to list pods in"),
		),
	),
	func (ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		namespace, err := requiredParam[string](request, "namespace")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client, err := getClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get Kubernetes client: %w", err)
		}

		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list pods: %w", err)
		}
		r, err := json.Marshal(pods)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal pods: %w", err)
		}

		return mcp.NewToolResultText(string(r)), nil
	}
}
