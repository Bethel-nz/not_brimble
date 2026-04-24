package pipeline

import (
	"context"
	"fmt"

	"not_brimble/internal/caddy"
	"not_brimble/internal/db"
	"not_brimble/internal/events"
)

type RouteHandler struct {
	DB    *db.DB
	Bus   *events.Bus
	Caddy *caddy.Client
}

func (h *RouteHandler) Handle(ctx context.Context, evt events.PipelineEvent) error {
	id := evt.DeploymentID

	dep, err := h.DB.GetDeployment(ctx, id)
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	subdomain := dep.Name + ".localhost"
	h.DB.AppendLogLine(ctx, id, "stdout",
		fmt.Sprintf("[route] registering subdomain %s", subdomain))

	if err := h.Caddy.AddRoute(ctx, caddy.Route{
		DeploymentID: dep.Name,
		Host:         subdomain,
		Upstream:     Upstream(dep.Name),
	}); err != nil {
		dep.Status = db.StatusFailed
		h.DB.UpdateDeployment(ctx, dep)
		h.Bus.Publish(ctx, events.QueueFailed, events.PipelineEvent{
			DeploymentID: id, Stage: "failed",
			ErrorMsg: fmt.Sprintf("caddy route: %v", err),
		})
		return nil
	}

	dep.Subdomain = subdomain
	dep.CaddyRouteID = "deploy-" + dep.Name
	dep.Status = db.StatusRunning
	h.DB.UpdateDeployment(ctx, dep)

	h.DB.AppendLogLine(ctx, id, "stdout",
		fmt.Sprintf("[route] live at http://%s", subdomain))

	// Retire previously-running siblings for the same source_url now that
	// the new one is serving traffic. We enqueue pipeline.delete rather
	// than call the Docker/Caddy clients directly — the DeleteHandler
	// already owns container stop + route teardown + status transition,
	// so reusing it keeps the cleanup path in one place.
	if dep.SourceURL != "" {
		siblings, err := h.DB.ListRunningBySourceURL(ctx, dep.SourceURL)
		if err == nil {
			for _, s := range siblings {
				if s.ID == dep.ID {
					continue
				}
				h.DB.AppendLogLine(ctx, id, "stdout",
					fmt.Sprintf("[route] retiring previous sibling %s", s.ID))
				h.Bus.Publish(ctx, events.QueueDelete, events.PipelineEvent{
					DeploymentID: s.ID,
					Stage:        "delete",
				})
			}
		}
	}

	return nil
}
