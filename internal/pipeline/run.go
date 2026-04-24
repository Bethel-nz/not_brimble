package pipeline

import (
	"context"
	"fmt"

	"not_brimble/internal/db"
	"not_brimble/internal/docker"
	"not_brimble/internal/events"
)

type RunHandler struct {
	DB     *db.DB
	Bus    *events.Bus
	Docker *docker.Client
}

func (h *RunHandler) Handle(ctx context.Context, evt events.PipelineEvent) error {
	id := evt.DeploymentID

	dep, err := h.DB.GetDeployment(ctx, id)
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	dep.Status = db.StatusDeploying
	h.DB.UpdateDeployment(ctx, dep)
	h.DB.AppendLogLine(ctx, id, "stdout",
		fmt.Sprintf("[run] starting container from image %s", evt.ImageTag))

	cname := ContainerName(dep.Name)
	containerID, err := h.Docker.Create(ctx, docker.CreateOptions{
		Image: evt.ImageTag,
		Name:  cname,
		Env:   []string{"PORT=3000"},
	})
	if err != nil {
		h.fail(ctx, dep, fmt.Sprintf("container create: %v", err))
		return nil
	}

	if err := h.Docker.Start(ctx, containerID); err != nil {
		h.fail(ctx, dep, fmt.Sprintf("container start: %v", err))
		return nil
	}

	dep.ContainerID = containerID
	h.DB.UpdateDeployment(ctx, dep)

	h.DB.AppendLogLine(ctx, id, "stdout", fmt.Sprintf("[run] container %s started", cname))

	return h.Bus.Publish(ctx, events.QueueRunning, events.PipelineEvent{
		DeploymentID: id,
		Stage:        "running",
		ImageTag:     evt.ImageTag,
		ContainerID:  containerID,
	})
}

func (h *RunHandler) fail(ctx context.Context, dep db.Deployment, msg string) {
	dep.Status = db.StatusFailed
	h.DB.UpdateDeployment(ctx, dep)
	h.Bus.Publish(ctx, events.QueueFailed, events.PipelineEvent{
		DeploymentID: dep.ID,
		Stage:        "failed",
		ErrorMsg:     msg,
	})
}

// ContainerName wraps the deployment slug with a prefix so deployment
// containers are easy to spot alongside other containers on the host.
func ContainerName(name string) string {
	return "deploy-" + name
}

// Upstream returns the dial target Caddy should reverse-proxy to for a given
// deployment slug. Exported so the worker's startup-sync path and the live
// RouteHandler agree on how to address the container.
func Upstream(name string) string {
	return ContainerName(name) + ":3000"
}
