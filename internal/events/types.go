package events

import "encoding/json"

const (
	QueueQueued  = "pipeline.queued"
	QueueBuilt   = "pipeline.built"
	QueueRunning = "pipeline.running"
	QueueFailed  = "pipeline.failed"
	QueueDelete  = "pipeline.delete"
)

type PipelineEvent struct {
	DeploymentID string `json:"deployment_id"`
	Stage        string `json:"stage"`
	ImageTag     string `json:"image_tag,omitempty"`
	ContainerID  string `json:"container_id,omitempty"`
	ErrorMsg     string `json:"error_msg,omitempty"`
	Retries      int    `json:"retries,omitempty"`
}

func (e PipelineEvent) Encode() string {
	b, _ := json.Marshal(e)
	return string(b)
}

func Decode(s string) (PipelineEvent, error) {
	var e PipelineEvent
	return e, json.Unmarshal([]byte(s), &e)
}

