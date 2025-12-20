package runtime

import (
	"context"
)

type AgentInfo struct {
	ID     string
	Name   string
	Status string
	Image  string
}

type RunConfig struct {
	Name        string
	Image       string
	HomeDir     string
	Workspace   string
	Env         []string
	Labels      map[string]string
}

type Runtime interface {
	RunDetached(ctx context.Context, config RunConfig) (string, error)
	Stop(ctx context.Context, id string) error
	List(ctx context.Context, labelFilter map[string]string) ([]AgentInfo, error)
	GetLogs(ctx context.Context, id string) (string, error)
}
