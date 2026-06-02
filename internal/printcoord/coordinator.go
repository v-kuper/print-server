package printcoord

import (
	"context"
	"sync"
)

type Coordinator struct {
	mu        sync.Mutex
	waitMu    sync.Mutex
	userQueue int
}

func New() *Coordinator {
	return &Coordinator{}
}

func (c *Coordinator) RunUserPrint(ctx context.Context, fn func(context.Context) error) error {
	if c == nil {
		return fn(ctx)
	}
	c.waitMu.Lock()
	c.userQueue++
	c.waitMu.Unlock()

	c.mu.Lock()
	c.waitMu.Lock()
	c.userQueue--
	c.waitMu.Unlock()
	defer c.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(ctx)
}

func (c *Coordinator) TryRunFax(ctx context.Context, fn func(context.Context) error) (bool, error) {
	if c == nil {
		return true, fn(ctx)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if c.UserPrintsWaiting() {
		return false, nil
	}
	if !c.mu.TryLock() {
		return false, nil
	}
	defer c.mu.Unlock()
	if c.UserPrintsWaiting() {
		return false, nil
	}
	if err := ctx.Err(); err != nil {
		return true, err
	}
	return true, fn(ctx)
}

func (c *Coordinator) UserPrintsWaiting() bool {
	if c == nil {
		return false
	}
	c.waitMu.Lock()
	defer c.waitMu.Unlock()
	return c.userQueue > 0
}
