package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
)

type Client struct {
	cli         *dockerclient.Client
	networkName string
}

func NewClient(networkName string) (*Client, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Client{cli: cli, networkName: networkName}, nil
}

type CreateOptions struct {
	Image string
	Name  string
	Env   []string
}

// Create creates and starts a container, returning its ID.
func (c *Client) Create(ctx context.Context, opts CreateOptions) (string, error) {
	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image: opts.Image,
			Env:   opts.Env,
		},
		&container.HostConfig{
			NetworkMode: container.NetworkMode(c.networkName),
		},
		&network.NetworkingConfig{},
		nil,
		opts.Name,
	)
	if err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}
	return resp.ID, nil
}

// Start starts a container by ID.
func (c *Client) Start(ctx context.Context, containerID string) error {
	return c.cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

// Stop stops a container gracefully.
func (c *Client) Stop(ctx context.Context, containerID string) error {
	return c.cli.ContainerStop(ctx, containerID, container.StopOptions{})
}

// Remove removes a stopped container.
func (c *Client) Remove(ctx context.Context, containerID string) error {
	return c.cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
}

// IsRunning reports whether the given container exists and is in the "running"
// state. A missing container or a non-running state (exited, created, dead)
// returns false with no error — callers use this to decide whether it's safe
// to register a Caddy route that would otherwise 502.
func (c *Client) IsRunning(ctx context.Context, nameOrID string) (bool, error) {
	info, err := c.cli.ContainerInspect(ctx, nameOrID)
	if err != nil {
		if dockerclient.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return info.State != nil && info.State.Running, nil
}
