package pipeline

import (
	"context"
	"fmt"
	"strings"

	"not_brimble/internal/caddy"
	"not_brimble/internal/db"
	"not_brimble/internal/docker"
	"not_brimble/internal/events"
)

// DeleteHandler stops and removes the container for a deployment and tears
// down its Caddy route. It is idempotent: a missing container or route is
// treated as success.
type DeleteHandler struct {
	DB     *db.DB
	Bus    *events.Bus
	Docker *docker.Client
	Caddy  *caddy.Client
}

func (h *DeleteHandler) Handle(ctx context.Context, evt events.PipelineEvent) error {
	id := evt.DeploymentID

	dep, err := h.DB.GetDeployment(ctx, id)
	if err == db.ErrNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	h.DB.AppendLogLine(ctx, id, "stdout", "[delete] tearing down")

	if dep.ContainerID != "" {
		h.Docker.Stop(ctx, dep.ContainerID)
		h.Docker.Remove(ctx, dep.ContainerID)
	}

	if dep.CaddyRouteID != "" {
		// DeleteRoute takes the ID fragment, not the full @id.
		routeID := strings.TrimPrefix(dep.CaddyRouteID, "deploy-")
		h.Caddy.DeleteRoute(ctx, routeID)
	}

	dep.Status = db.StatusStopped
	h.DB.UpdateDeployment(ctx, dep)

	h.DB.AppendLogLine(ctx, id, "stdout", "[delete] done")
	return nil
}
