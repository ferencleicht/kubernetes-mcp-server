package kubernetes

import (
	"context"

	"github.com/ferencleicht/kubernetes-mcp-server/pkg/toolsets"
	"k8s.io/client-go/kubernetes"
)

type GetClientFn func(context.Context) (*kubernetes.Clientset, error)

var DefaultToolsets = []string{"all"}

func InitToolsets(passedToolsets []string, readonly bool, getClient GetClientFn) (*toolsets.ToolsetGroup, error) {
	tsg := toolsets.NewToolsetGroup(readonly)

	pods := toolsets.NewToolset("pods", "Kubernetes Pod related tools").
		AddReadTools(
			toolsets.NewServerTool(ListPods(getClient)),
		)

	services := toolsets.NewToolset("services", "Kubernetes Service related tools").
		AddReadTools(
			toolsets.NewServerTool(ListServices(getClient)),
		)

	tsg.AddToolset(pods)
	tsg.AddToolset(services)

	if err := tsg.EnableToolsets(passedToolsets); err != nil {
		return nil, err
	}

	return tsg, nil
}
