package pool

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

var ErrPoolStopped = errors.New("pool has stopped accepting tasks")

type Task[T any] struct {
	ID   int
	Func func(ctx context.Context) (T, error)
}

type Result[T any] struct {
	ID    int
	Value *T
	Error error
}

// Legacy cancellable pool implementation (kept commented for comparison).
// It cancels unfinished work but retains every result produced by a task that returns.
//
// type Pool[T any] struct {
// 	taskChan chan Task[T]
//
// 	wg      sync.WaitGroup
// 	stopped sync.Once
//
// 	resultsMu sync.Mutex
// 	results   []Result[T]
//
// 	ctx     context.Context
// 	cancel  context.CancelFunc
// }
//
// func NewPool[T any](ctx context.Context, workerCount, queueSize int) *Pool[T] {
// 	if workerCount <= 0 {
// 		workerCount = 1
// 	}
// 	if queueSize <= 0 {
// 		queueSize = 1
// 	}
// 	if ctx == nil {
// 		ctx = context.Background()
// 	}
//
// 	poolCtx, cancel := context.WithCancel(ctx)
// 	p := &Pool[T]{
// 		taskChan: make(chan Task[T], queueSize),
// 		results:  make([]Result[T], 0, workerCount+queueSize),
// 		ctx:      poolCtx,
// 		cancel:   cancel,
// 	}
// 	p.wg.Add(workerCount)
// 	for i := 0; i < workerCount; i++ {
// 		go p.worker()
// 	}
// 	return p
// }
//
// func (p *Pool[T]) Stop() {
// 	p.stopped.Do(func() {
// 		p.cancel()
// 		p.wg.Wait()
// 	})
// }
//
// func (p *Pool[T]) AddTask(id int, fn func(ctx context.Context) (T, error)) error {
// 	// taskChan is intentionally never closed. Closing it would race with
// 	// concurrent producers and turn a best-effort submission into a panic.
// 	if p.ctx.Err() != nil {
// 		return ErrPoolStopped
// 	}
// 	select {
// 	case p.taskChan <- Task[T]{ID: id, Func: fn}:
// 		return nil
// 	case <-p.ctx.Done():
// 		return ErrPoolStopped
// 	}
// }
//
// func (p *Pool[T]) GetResults() <-chan Result[T] {
// 	results := make(chan Result[T])
// 	go func() {
// 		defer close(results)
// 		for _, result := range p.CollectResults() {
// 			results <- result
// 		}
// 	}()
// 	return results
// }
//
// func (p *Pool[T]) CollectResults() []Result[T] {
// 	p.Stop()
//
// 	p.resultsMu.Lock()
// 	defer p.resultsMu.Unlock()
//
// 	results := make([]Result[T], len(p.results))
// 	copy(results, p.results)
// 	return results
// }
//
// func (p *Pool[T]) worker() {
// 	defer p.wg.Done()
//
// 	for {
// 		// Prefer cancellation over taking another queued task. The second
// 		// cancellation case handles cancellation while waiting for work.
// 		select {
// 		case <-p.ctx.Done():
// 			return
// 		default:
// 		}
//
// 		select {
// 		case <-p.ctx.Done():
// 			return
// 		case task := <-p.taskChan:
// 			select {
// 			case <-p.ctx.Done():
// 				return
// 			default:
// 			}
// 			result := runTask(p.ctx, task)
//
// 			p.resultsMu.Lock()
// 			p.results = append(p.results, result)
// 			p.resultsMu.Unlock()
// 		}
// 	}
// }
//
// func runTask[T any](ctx context.Context, task Task[T]) (result Result[T]) {
// 	result.ID = task.ID
// 	defer func() {
// 		if recovered := recover(); recovered != nil {
// 			result.Value = nil
// 			result.Error = fmt.Errorf("panic: %v", recovered)
// 		}
// 	}()
//
// 	value, err := task.Func(ctx)
// 	result.Value = &value
// 	result.Error = err
// 	return result
// }

// Pool executes every accepted task and records exactly one result for it.
// A task error is data in Result and never stops the other workers.
type Pool[T any] struct {
	taskChan chan Task[T]

	group    errgroup.Group
	stopOnce sync.Once

	submitMu sync.RWMutex
	stopped  bool

	resultsMu sync.Mutex
	results   []Result[T]
}

func NewPool[T any](ctx context.Context, workerCount, queueSize int) *Pool[T] {
	if workerCount <= 0 {
		workerCount = 1
	}
	if queueSize <= 0 {
		queueSize = 1
	}
	if ctx == nil {
		ctx = context.Background()
	}

	p := &Pool[T]{
		taskChan: make(chan Task[T], queueSize),
		results:  make([]Result[T], 0, workerCount+queueSize),
	}
	for i := 0; i < workerCount; i++ {
		p.group.Go(func() error {
			return p.worker(ctx)
		})
	}
	return p
}

// AddTask submits a task unless the pool has already been stopped.
// It applies backpressure when the task queue is full.
func (p *Pool[T]) AddTask(id int, fn func(ctx context.Context) (T, error)) error {
	p.submitMu.RLock()
	defer p.submitMu.RUnlock()

	if p.stopped {
		return ErrPoolStopped
	}
	p.taskChan <- Task[T]{ID: id, Func: fn}
	return nil
}

// Stop stops accepting tasks and waits for every accepted task to finish.
// It does not cancel the task context and does not stop on individual errors.
func (p *Pool[T]) Stop() {
	p.stopOnce.Do(func() {
		p.submitMu.Lock()
		p.stopped = true
		close(p.taskChan)
		p.submitMu.Unlock()

		// Workers always return nil. Task failures are retained in Result so one
		// failed task cannot cancel or otherwise short-circuit the remaining work.
		_ = p.group.Wait()
	})
}

// CollectResults waits for all accepted tasks and returns a result snapshot.
func (p *Pool[T]) CollectResults() []Result[T] {
	p.Stop()

	p.resultsMu.Lock()
	defer p.resultsMu.Unlock()

	results := make([]Result[T], len(p.results))
	copy(results, p.results)
	return results
}

// GetResults returns all results through a channel and closes it afterwards.
func (p *Pool[T]) GetResults() <-chan Result[T] {
	results := make(chan Result[T])
	go func() {
		defer close(results)
		for _, result := range p.CollectResults() {
			results <- result
		}
	}()
	return results
}

func (p *Pool[T]) worker(ctx context.Context) error {
	for task := range p.taskChan {
		result := runTask(ctx, task)

		p.resultsMu.Lock()
		p.results = append(p.results, result)
		p.resultsMu.Unlock()
	}
	return nil
}

func runTask[T any](ctx context.Context, task Task[T]) (result Result[T]) {
	result.ID = task.ID
	defer func() {
		if recovered := recover(); recovered != nil {
			result.Value = nil
			result.Error = fmt.Errorf("panic: %v", recovered)
		}
	}()

	value, err := task.Func(ctx)
	result.Value = &value
	result.Error = err
	return result
}
