package events

import (
	"context"

	"github.com/nalgeon/redka"
)

type Bus struct {
	rdb *redka.DB
}

func NewBus(rdb *redka.DB) *Bus {
	return &Bus{rdb: rdb}
}

// Publish pushes a pipeline event to the named queue.
func (b *Bus) Publish(ctx context.Context, queue string, evt PipelineEvent) error {
	_, err := b.rdb.List().PushBack(queue, evt.Encode())
	return err
}
