package watch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	"github.com/fsnotify/fsnotify"
)

var _ workspace.FileWatcher = (*Watcher)(nil)

const (
	defaultBufferSize = 64
	defaultDebounce   = 75 * time.Millisecond
)

type Watcher struct {
	mu       sync.Mutex
	root     string
	rootID   string
	watcher  *fsnotify.Watcher
	ctx      context.Context
	cancel   context.CancelFunc
	changes  chan workspace.FileChange
	stopped  chan struct{}
	running  bool
	debounce time.Duration
}

func New() *Watcher {
	return &Watcher{debounce: defaultDebounce}
}

func (w *Watcher) Start(ctx context.Context, root string) (<-chan workspace.FileChange, error) {
	if ctx == nil {
		return nil, workspace.NewError(workspace.ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, workspace.CanceledError(err)
	}
	canonicalRoot, err := workspace.CanonicalRoot(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(canonicalRoot, 0o700); err != nil {
		return nil, workspace.WrapError(workspace.ErrStorageUnavailable, "prepare watcher root", err)
	}
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, workspace.WrapError(workspace.ErrStorageUnavailable, "create filesystem watcher", err)
	}
	if err := addDirectories(fsWatcher, canonicalRoot); err != nil {
		_ = fsWatcher.Close()
		return nil, workspace.WrapError(workspace.ErrStorageUnavailable, "register watcher directories", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		_ = fsWatcher.Close()
		return nil, workspace.NewError(workspace.ErrConflict, "workspace watcher is already running")
	}
	watchCtx, cancel := context.WithCancel(ctx)
	w.root = canonicalRoot
	w.rootID = canonicalRoot
	w.watcher = fsWatcher
	w.ctx = watchCtx
	w.cancel = cancel
	w.changes = make(chan workspace.FileChange, defaultBufferSize)
	w.stopped = make(chan struct{})
	w.running = true
	go w.loop()
	return w.changes, nil
}

func (w *Watcher) Stop() error {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return nil
	}
	cancel := w.cancel
	stopped := w.stopped
	w.running = false
	w.cancel = nil
	w.mu.Unlock()
	cancel()
	<-stopped
	return nil
}

func (w *Watcher) loop() {
	w.mu.Lock()
	ctx := w.ctx
	fsWatcher := w.watcher
	changes := w.changes
	stopped := w.stopped
	root := w.root
	debounce := w.debounce
	w.mu.Unlock()
	defer close(stopped)
	defer fsWatcher.Close()
	defer close(changes)

	pending := make(map[string]workspace.FileChange)
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	flush := func() {
		for _, change := range pending {
			select {
			case changes <- change:
			default:
				pending = map[string]workspace.FileChange{
					"": {ID: eventID(), CorrelationID: string(eventID()), RootID: root, Operation: workspace.FileRescanRequired, ObservedAt: time.Now().UTC(), ErrorCode: string(workspace.ErrWatcherOverflow)},
				}
				return
			}
		}
		pending = make(map[string]workspace.FileChange)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			flush()
		case event, ok := <-fsWatcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create != 0 {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					_ = addDirectories(fsWatcher, event.Name)
				}
			}
			change, accepted := w.toChange(root, event)
			if !accepted {
				continue
			}
			pending[change.RelativePath] = change
			timer.Reset(debounce)
		case watchErr, ok := <-fsWatcher.Errors:
			if !ok {
				return
			}
			pending[""] = workspace.FileChange{ID: eventID(), CorrelationID: string(eventID()), RootID: root, Operation: workspace.FileError, ObservedAt: time.Now().UTC(), ErrorCode: string(workspace.ErrStorageUnavailable)}
			timer.Reset(debounce)
			_ = watchErr
		}
	}
}

func (w *Watcher) toChange(root string, event fsnotify.Event) (workspace.FileChange, bool) {
	relative, err := filepath.Rel(root, event.Name)
	if err != nil || relative == "." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return workspace.FileChange{}, false
	}
	if err := verifyEventContainment(root, event.Name); err != nil {
		return workspace.FileChange{}, false
	}
	operation := workspace.FileModified
	switch {
	case event.Op&fsnotify.Create != 0:
		operation = workspace.FileCreated
	case event.Op&fsnotify.Remove != 0:
		operation = workspace.FileRemoved
	case event.Op&fsnotify.Rename != 0:
		operation = workspace.FileRenamed
	}
	return workspace.FileChange{ID: eventID(), CorrelationID: string(eventID()), RootID: root, RelativePath: filepath.ToSlash(relative), Operation: operation, ObservedAt: time.Now().UTC()}, true
}

func verifyEventContainment(root, candidate string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	ancestor := candidate
	for {
		resolved, resolveErr := filepath.EvalSymlinks(ancestor)
		if resolveErr == nil {
			relative, relativeErr := filepath.Rel(resolvedRoot, resolved)
			if relativeErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
				return workspace.NewError(workspace.ErrPathDenied, "watch event resolves outside approved root")
			}
			return nil
		}
		if !os.IsNotExist(resolveErr) {
			return resolveErr
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return workspace.NewError(workspace.ErrPathDenied, "watch event cannot be contained")
		}
		ancestor = parent
	}
}

func addDirectories(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if err := watcher.Add(path); err != nil {
			return err
		}
		return nil
	})
}

func eventID() workspace.EventID {
	return workspace.EventID(time.Now().UTC().Format("20060102T150405.000000000Z07:00"))
}
