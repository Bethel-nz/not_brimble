package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"

	"not_brimble/internal/db"
	"not_brimble/internal/events"
	"not_brimble/internal/notify"
)

type Handler struct {
	db        *db.DB
	bus       *events.Bus
	uploadDir string
	broker    *notify.Broker
}

// CreateDeployment accepts either a JSON body ({"source_type":"git",...})
// or a multipart upload (source_type=upload, file=<tarball>), records a new
// deployment row, and publishes a pipeline.queued event for the worker.
// @Summary Create a new deployment
// @Description Create a deployment from a Git URL or a .tar.gz upload
// @Accept json,mpfd
// @Produce json
// @Param sourceType formData string true "Source type (git or upload)"
// @Param sourceUrl formData string false "Git URL (required if sourceType is git)"
// @Param file formData file false "Source archive (required if sourceType is upload)"
// @Success 201 {object} db.Deployment
// @Router /deployments [post]
func (h *Handler) CreateDeployment(c *gin.Context) {
	id := ulid.Make().String()
	var sourceType, sourceURL string

	ct := c.GetHeader("Content-Type")
	if strings.HasPrefix(strings.ToLower(ct), "application/json") {
		var req struct {
			SourceType string `json:"source_type"`
			SourceURL  string `json:"source_url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}
		sourceType = req.SourceType
		sourceURL = req.SourceURL
	} else {
		sourceType = c.PostForm("source_type")
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file required for upload"})
			return
		}
		dest := filepath.Join(h.uploadDir, id+".tar.gz")
		if err := c.SaveUploadedFile(file, dest); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save upload"})
			return
		}
		sourceURL = dest
	}

	switch sourceType {
	case "git":
		if sourceURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "source_url required for git"})
			return
		}
		// Reject a URL without a scheme up front so the user sees a clear
		// error instead of discovering it 30s later in build logs (git
		// interprets "github.com/foo/bar" as a local path and fails with
		// a confusing "repository does not exist").
		if !strings.HasPrefix(sourceURL, "https://") &&
			!strings.HasPrefix(sourceURL, "http://") &&
			!strings.HasPrefix(sourceURL, "git@") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "source_url must start with https://, http://, or git@"})
			return
		}
	case "upload":
		// sourceURL was set above from the multipart save
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_type must be 'git' or 'upload'"})
		return
	}

	dep := db.Deployment{
		ID:         id,
		Name:       deriveName(sourceType, sourceURL, id),
		SourceType: sourceType,
		SourceURL:  sourceURL,
		Status:     db.StatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := h.db.CreateDeployment(c.Request.Context(), dep); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	h.bus.Publish(c.Request.Context(), events.QueueQueued, events.PipelineEvent{
		DeploymentID: id,
		Stage:        "queued",
	})

	c.JSON(http.StatusCreated, dep)
}

// @Summary List all deployments
// @Description Get a list of all deployments in the system
// @Produce json
// @Success 200 {array} db.Deployment
// @Router /deployments [get]
func (h *Handler) ListDeployments(c *gin.Context) {
	deps, err := h.db.ListDeployments(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if deps == nil {
		deps = []db.Deployment{}
	}
	c.JSON(http.StatusOK, deps)
}

// @Summary Get deployment details
// @Description Get detailed information about a specific deployment by ID
// @Produce json
// @Param id path string true "Deployment ID"
// @Success 200 {object} db.Deployment
// @Router /deployments/{id} [get]
func (h *Handler) GetDeployment(c *gin.Context) {
	id := c.Param("id")
	dep, err := h.db.GetDeployment(c.Request.Context(), id)
	if err == db.ErrNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	c.JSON(http.StatusOK, dep)
}

// RedeployDeployment takes the image_tag from an existing deployment and
// creates a brand new deployment that reuses it — skipping the build step
// entirely. The new deployment gets its own ULID, name, and subdomain,
// and shows up as a distinct entry in the list. This is the "rollback"
// or "redeploy this version" action.
// @Summary Redeploy an existing deployment
// @Description Create a new deployment using the image from an existing deployment (Rollback)
// @Produce json
// @Param id path string true "Source Deployment ID"
// @Success 201 {object} db.Deployment
// @Router /deployments/{id}/redeploy [post]
func (h *Handler) RedeployDeployment(c *gin.Context) {
	sourceID := c.Param("id")
	source, err := h.db.GetDeployment(c.Request.Context(), sourceID)
	if err == db.ErrNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if source.ImageTag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source has no image to redeploy"})
		return
	}

	newID := ulid.Make().String()
	dep := db.Deployment{
		ID: newID,
		// Derive the name as if this were a fresh deploy of the same
		// source so upload-originated rollbacks still get a clean slug.
		Name:       deriveName(source.SourceType, source.SourceURL, newID),
		SourceType: "rollback",
		SourceURL:  source.SourceURL,
		ImageTag:   source.ImageTag,
		Status:     db.StatusBuilt,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := h.db.CreateDeployment(c.Request.Context(), dep); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	// Rollbacks use source.ImageTag directly, so image_tag isn't written
	// by the build handler
	h.db.UpdateDeployment(c.Request.Context(), dep)

	h.db.AppendLogLine(c.Request.Context(), newID, "stdout",
		fmt.Sprintf("[rollback] redeploying %s (from %s)", source.ImageTag, source.ID))

	// Skip straight to the run stage — the image already exists.
	h.bus.Publish(c.Request.Context(), events.QueueBuilt, events.PipelineEvent{
		DeploymentID: newID,
		Stage:        "built",
		ImageTag:     source.ImageTag,
	})

	c.JSON(http.StatusCreated, dep)
}

// DeleteDeployment publishes a pipeline.delete event. The worker owns the
// side effects (stopping the container, removing the Caddy route) so that
// the API process never needs access to the Docker socket.
// @Summary Delete a deployment
// @Description Enqueue a deployment for deletion (soft delete)
// @Param id path string true "Deployment ID"
// @Success 202 "Accepted"
// @Router /deployments/{id} [delete]
func (h *Handler) DeleteDeployment(c *gin.Context) {
	id := c.Param("id")
	if _, err := h.db.GetDeployment(c.Request.Context(), id); err == db.ErrNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if err := h.bus.Publish(c.Request.Context(), events.QueueDelete, events.PipelineEvent{
		DeploymentID: id,
		Stage:        "delete",
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "enqueue delete"})
		return
	}

	c.Status(http.StatusAccepted)
}
