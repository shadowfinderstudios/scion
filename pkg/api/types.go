package api

import (
	"context"
)

type AgentConfig struct {
	Grove      string            `json:"grove"`
	Name       string            `json:"name"`
	Status     string            `json:"status,omitempty"`
	Kubernetes *AgentK8sMetadata `json:"kubernetes,omitempty"`
}

type AgentK8sMetadata struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	PodName   string `json:"podName"`
	SyncedAt  string `json:"syncedAt,omitempty"`
}

type VolumeMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

type KubernetesConfig struct {
	Context          string        `json:"context,omitempty"`
	Namespace        string        `json:"namespace,omitempty"`
	RuntimeClassName string        `json:"runtimeClassName,omitempty"`
	Resources        *K8sResources `json:"resources,omitempty"`
}

type K8sResources struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

type ScionConfig struct {
	Template        string            `json:"template"`
	HarnessProvider string            `json:"harness_provider,omitempty"`
	ConfigDir       string            `json:"config_dir,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	Volumes         []VolumeMount     `json:"volumes,omitempty"`
	UnixUsername    string            `json:"unix_username"`
	Image           string            `json:"image"`
	Detached        *bool             `json:"detached"`
	UseTmux         *bool             `json:"use_tmux"`
	Model           string            `json:"model"`
	Runtime         string            `json:"runtime,omitempty"`
	Kubernetes      *KubernetesConfig `json:"kubernetes,omitempty"`
	Agent           *AgentConfig      `json:"agent,omitempty"`
}

func (c *ScionConfig) IsDetached() bool {
	if c.Detached == nil {
		return true
	}
	return *c.Detached
}

func (c *ScionConfig) IsUseTmux() bool {
	if c.UseTmux == nil {
		return false
	}
	return *c.UseTmux
}

type AuthConfig struct {
	GeminiAPIKey         string
	GoogleAPIKey         string
	VertexAPIKey         string
	GoogleAppCredentials string
	GoogleCloudProject   string
	OAuthCreds           string
	AnthropicAPIKey      string
}

type AuthProvider interface {
	GetAuthConfig(context.Context) (AuthConfig, error)
}

type AgentInfo struct {
	ID          string
	Name        string
	Template    string
	Grove       string
	GrovePath   string
	Labels      map[string]string
	Annotations map[string]string
	Status      string // Container status
	AgentStatus string // Scion agent high-level status
	Image       string
	Detached    bool
}

type StartOptions struct {
	Name      string
	Task      string
	Template  string
	Image     string
	GrovePath string
	Env       map[string]string
	Detached  *bool
	Resume    bool
	Model     string
	Auth      AuthProvider
	NoAuth    bool
}

type StatusEvent struct {
	AgentID   string    `json:"agent_id"`
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	Timestamp string    `json:"timestamp"`
}
