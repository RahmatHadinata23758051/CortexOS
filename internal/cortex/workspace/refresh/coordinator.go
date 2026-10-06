package refresh

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

const (
	defaultDebounce = 75 * time.Millisecond
	channelCapacity = 64
)

type Request struct {
	ProjectID    workspace.ProjectID
	RootID       string
	RelativePath string
	Operation    workspace.FileChangeOperation
	ContentHash  string
	ObservedAt   time.Time
}

type Failure struct {
	Request Request
	Error   error
}

type Handler func(context.Context, Request) error

type Coordinator struct {
	watcher  workspace.FileWatcher
	handler  Handler
	debounce time.Duration

	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc
	stopped  chan struct{}
	requests chan Request
	failures chan Failure
}

func New(watcher workspace.FileWatcher, handler Handler) (*Coordinator, error) {
	if watcher == nil || handler == nil {
		return nil, workspace.NewError(workspace.ErrInvalidRequest, "watcher and refresh handler are required")
	}
	return &Coordinator{watcher: watcher, handler: handler, debounce: defaultDebounce}, nil
}

func (c *Coordinator) Requests() <-chan Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.requests == nil {
		c.requests = make(chan Request, channelCapacity)
	}
	return c.requests
}

func (c *Coordinator) Failures() <-chan Failure {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failures == nil {
		c.failures = make(chan Failure, channelCapacity)
	}
	return c.failures
}

func (c *Coordinator) Start(ctx context.Context, projectID workspace.ProjectID, root string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if projectID == "" {
		return workspace.NewError(workspace.ErrInvalidRequest, "project id is required")
	}
	changes, err := c.watcher.Start(ctx, root)
	if err != nil {
		return err
	}
	watchCtx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		cancel()
		_ = c.watcher.Stop()
		return workspace.NewError(workspace.ErrConflict, "refresh coordinator is already running")
	}
	c.running = true
	c.cancel = cancel
	c.stopped = make(chan struct{})
	if c.requests == nil {
		c.requests = make(chan Request, channelCapacity)
	}
	if c.failures == nil {
		c.failures = make(chan Failure, channelCapacity)
	}
	stopped := c.stopped
	c.mu.Unlock()
	go c.loop(watchCtx, projectID, changes, stopped)
	return nil
}

func (c *Coordinator) Stop() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	cancel := c.cancel
	stopped := c.stopped
	c.mu.Unlock()
	cancel()
	if err := c.watcher.Stop(); err != nil {
		return err
	}
	<-stopped
	return nil
}

func (c *Coordinator) loop(ctx context.Context, projectID workspace.ProjectID, changes <-chan workspace.FileChange, stopped chan struct{}) {
	defer func() {
		c.mu.Lock()
		c.running = false
		c.cancel = nil
		c.stopped = nil
		c.mu.Unlock()
		close(stopped)
	}()

	pending := make(map[string]Request)
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	flush := func() {
		items := make([]Request, 0, len(pending))
		for _, request := range pending {
			items = append(items, request)
		}
		sort.Slice(items, func(left, right int) bool { return items[left].RelativePath < items[right].RelativePath })
		for _, request := range items {
			c.publishRequest(request)
			if err := c.handler(ctx, request); err != nil {
				c.publishFailure(Failure{Request: request, Error: workspace.CanceledError(err)})
			}
		}
		pending = make(map[string]Request)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			flush()
		case change, ok := <-changes:
			if !ok {
				flush()
				return
			}
			if change.RelativePath == "" && change.Operation != workspace.FileRescanRequired && change.Operation != workspace.FileError {
				continue
			}
			pending[change.RelativePath] = Request{
				ProjectID: projectID, RootID: change.RootID, RelativePath: change.RelativePath,
				Operation: change.Operation, ContentHash: change.ContentHash, ObservedAt: change.ObservedAt,
			}
			timer.Reset(c.debounce)
		}
	}
}

func (c *Coordinator) publishRequest(request Request) {
	c.mu.Lock()
	requests := c.requests
	c.mu.Unlock()
	if requests == nil {
		return
	}
	select {
	case requests <- request:
	default:
		// Request delivery is an observability aid. The handler remains the
		// authoritative refresh path, so a full audit channel cannot block it.
	}
}

func (c *Coordinator) publishFailure(failure Failure) {
	c.mu.Lock()
	failures := c.failures
	c.mu.Unlock()
	if failures == nil {
		return
	}
	select {
	case failures <- failure:
	default:
	}
}

func NewRebuildHandler(index workspace.RetrievalIndex) Handler {
	return func(ctx context.Context, request Request) error {
		if index == nil {
			return workspace.NewError(workspace.ErrInvalidRequest, "retrieval index is required")
		}
		return index.Rebuild(ctx, request.ProjectID)
	}
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return workspace.NewError(workspace.ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return workspace.CanceledError(err)
	}
	return nil
}
