package pool

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCollectResultsWaitsForEveryTask(t *testing.T) {
	pool := NewPool[int](context.Background(), 3, 2)
	wantErr := errors.New("task failed")
	var calls atomic.Int32

	for id := 1; id <= 8; id++ {
		id := id
		pool.AddTask(id, func(context.Context) (int, error) {
			calls.Add(1)
			if id == 2 {
				return 0, wantErr
			}
			return id * 2, nil
		})
	}

	results := pool.CollectResults()
	if got := calls.Load(); got != 8 {
		t.Fatalf("executed %d tasks, want 8", got)
	}
	if len(results) != 8 {
		t.Fatalf("got %d results, want 8", len(results))
	}

	byID := indexResults(results)
	for id := 1; id <= 8; id++ {
		result, ok := byID[id]
		if !ok {
			t.Errorf("missing result for task %d", id)
			continue
		}
		if id == 2 {
			if !errors.Is(result.Error, wantErr) {
				t.Errorf("task 2 error is %v, want %v", result.Error, wantErr)
			}
			continue
		}
		if result.Error != nil {
			t.Errorf("task %d returned an unexpected error: %v", id, result.Error)
			continue
		}
		if result.Value == nil || *result.Value != id*2 {
			t.Errorf("task %d returned value %v, want %d", id, result.Value, id*2)
		}
	}
}

func TestCancelledContextStillProducesEveryResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pool := NewPool[int](ctx, 1, 4)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})

	pool.AddTask(1, func(ctx context.Context) (int, error) {
		close(firstStarted)
		<-releaseFirst
		return 0, ctx.Err()
	})
	for id := 2; id <= 5; id++ {
		pool.AddTask(id, func(ctx context.Context) (int, error) {
			return 0, ctx.Err()
		})
	}

	<-firstStarted
	cancel()
	close(releaseFirst)

	results := pool.CollectResults()
	if len(results) != 5 {
		t.Fatalf("got %d results after cancellation, want 5", len(results))
	}
	for _, result := range results {
		if !errors.Is(result.Error, context.Canceled) {
			t.Errorf("task %d error is %v, want context.Canceled", result.ID, result.Error)
		}
	}
}

func TestPanicProducesOneResultAndDoesNotStopWorkers(t *testing.T) {
	pool := NewPool[int](context.Background(), 1, 2)
	pool.AddTask(1, func(context.Context) (int, error) {
		panic("boom")
	})
	pool.AddTask(2, func(context.Context) (int, error) {
		return 42, nil
	})

	results := indexResults(pool.CollectResults())
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if result := results[1]; result.Value != nil || result.Error == nil || !strings.Contains(result.Error.Error(), "boom") {
		t.Errorf("unexpected panic result: %+v", result)
	}
	if result := results[2]; result.Value == nil || *result.Value != 42 || result.Error != nil {
		t.Errorf("unexpected result after panic: %+v", result)
	}
}

func TestGetResultsReturnsEveryResult(t *testing.T) {
	pool := NewPool[int](context.Background(), 2, 2)
	for id := 1; id <= 6; id++ {
		id := id
		pool.AddTask(id, func(context.Context) (int, error) {
			return id, nil
		})
	}

	results := make([]Result[int], 0, 6)
	for result := range pool.GetResults() {
		results = append(results, result)
	}
	if len(results) != 6 {
		t.Fatalf("got %d results, want 6", len(results))
	}
}

func TestConcurrentAddTaskAndStop(t *testing.T) {
	pool := NewPool[int](context.Background(), 4, 8)
	var producers sync.WaitGroup
	for producer := 0; producer < 20; producer++ {
		producers.Add(1)
		go func() {
			defer producers.Done()
			for id := 0; id < 100; id++ {
				pool.AddTask(id, func(context.Context) (int, error) {
					return id, nil
				})
			}
		}()
	}

	pool.Stop()
	producers.Wait()
	err := pool.AddTask(999, func(context.Context) (int, error) {
		return 999, nil
	})
	if !errors.Is(err, ErrPoolStopped) {
		t.Fatalf("AddTask after Stop returned %v, want ErrPoolStopped", err)
	}

	results := pool.CollectResults()
	if len(results) > 2000 {
		t.Fatalf("got %d results, want at most 2000", len(results))
	}
	for _, result := range results {
		if result.ID == 999 {
			t.Fatal("task submitted after Stop was executed")
		}
	}
}

func indexResults[T any](results []Result[T]) map[int]Result[T] {
	indexed := make(map[int]Result[T], len(results))
	for _, result := range results {
		indexed[result.ID] = result
	}
	return indexed
}
