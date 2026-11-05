package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// flakyRunner times out the first `timeouts` calls, then returns err when set
// or the scripted output for the command.
type flakyRunner struct {
	timeouts int
	err      error
	outputs  map[string][]byte
	onRun    func()
	calls    []string
}

func (r *flakyRunner) Run(_ context.Context, _ string, name string, args []string, _ []byte) ([]byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if r.onRun != nil {
		r.onRun()
	}
	if len(r.calls) <= r.timeouts {
		return nil, commandTimeoutError(name, args, time.Minute)
	}
	if r.err != nil {
		return nil, r.err
	}
	return r.outputs[r.calls[len(r.calls)-1]], nil
}

// retryRecorder builds an ExecBacklog whose backoff sleeps and retry logs are
// recorded instead of performed, so tests never really sleep.
type retryRecorder struct {
	sleeps []time.Duration
	logs   []string
}

func (rec *retryRecorder) backlog(r CommandRunner, retries int) ExecBacklog {
	return ExecBacklog{
		Runner:  r,
		Retries: retries,
		Sleep: func(_ context.Context, d time.Duration) error {
			rec.sleeps = append(rec.sleeps, d)
			return nil
		},
		Logf: func(f string, args ...any) { rec.logs = append(rec.logs, fmt.Sprintf(f, args...)) },
	}
}

const viewTaskOneJSON = `{"task":{"id":"TASK-1","title":"T","status":"To Do"}}`

func TestExecBacklogRetriesTimedOutViewThenSucceeds(t *testing.T) {
	r := &flakyRunner{timeouts: 2, outputs: map[string][]byte{"backlog task view TASK-1 --json": []byte(viewTaskOneJSON)}}
	rec := &retryRecorder{}
	resp, err := rec.backlog(r, 2).ViewTask(context.Background(), "/repo", "TASK-1")
	if err != nil || resp.Task.ID != "TASK-1" {
		t.Fatalf("ViewTask=%+v err=%v, want TASK-1 after retries", resp, err)
	}
	if len(r.calls) != 3 {
		t.Fatalf("attempts=%d, want 3: %v", len(r.calls), r.calls)
	}
	if fmt.Sprint(rec.sleeps) != "[2s 4s]" {
		t.Fatalf("backoff sleeps=%v, want [2s 4s]", rec.sleeps)
	}
	wantLogs := []string{
		"warning: backlog task view TASK-1 --json: timed out after 1m0s; retrying (1/2)",
		"warning: backlog task view TASK-1 --json: timed out after 1m0s; retrying (2/2)",
	}
	if strings.Join(rec.logs, "\n") != strings.Join(wantLogs, "\n") {
		t.Fatalf("retry logs=%q, want %q", rec.logs, wantLogs)
	}
}

func TestExecBacklogRetriesEveryReadOnlyCall(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		output  string
		call    func(ExecBacklog) error
	}{
		{"ListTasks", "backlog task list --json --max-count 100 --skip 0", `{"tasks":[]}`, func(b ExecBacklog) error {
			_, err := b.ListTasks(context.Background(), "/repo", 100, 0)
			return err
		}},
		{"ViewTask", "backlog task view TASK-1 --json", viewTaskOneJSON, func(b ExecBacklog) error {
			_, err := b.ViewTask(context.Background(), "/repo", "TASK-1")
			return err
		}},
		{"Statuses", "backlog config get statuses", "To Do, Done\n", func(b ExecBacklog) error {
			_, err := b.Statuses(context.Background(), "/repo")
			return err
		}},
		{"TaskPrefix", "backlog config get taskPrefix", "task\n", func(b ExecBacklog) error {
			_, err := b.TaskPrefix(context.Background(), "/repo")
			return err
		}},
		{"Milestones", "backlog milestone list --plain --show-completed", "m-1: One\n", func(b ExecBacklog) error {
			_, err := b.Milestones(context.Background(), "/repo")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &flakyRunner{timeouts: 1, outputs: map[string][]byte{tc.command: []byte(tc.output)}}
			rec := &retryRecorder{}
			if err := tc.call(rec.backlog(r, 2)); err != nil {
				t.Fatalf("%s after one timeout: %v", tc.name, err)
			}
			if len(r.calls) != 2 || r.calls[0] != tc.command || r.calls[1] != tc.command {
				t.Fatalf("calls=%v, want %q twice", r.calls, tc.command)
			}
		})
	}
}

func TestExecBacklogReturnsTimeoutWhenRetriesExhausted(t *testing.T) {
	r := &flakyRunner{timeouts: 10}
	rec := &retryRecorder{}
	_, err := rec.backlog(r, 2).ViewTask(context.Background(), "/repo", "TASK-1")
	if !errors.Is(err, errCommandTimeout) || err.Error() != "backlog task view TASK-1 --json: timed out after 1m0s" {
		t.Fatalf("err=%v, want the last timeout error", err)
	}
	if len(r.calls) != 3 || len(rec.sleeps) != 2 || len(rec.logs) != 2 {
		t.Fatalf("attempts=%d sleeps=%v logs=%d, want 3 attempts, 2 sleeps, 2 logs", len(r.calls), rec.sleeps, len(rec.logs))
	}
}

func TestExecBacklogRetryZeroDisablesRetries(t *testing.T) {
	r := &flakyRunner{timeouts: 1}
	rec := &retryRecorder{}
	if _, err := rec.backlog(r, 0).ViewTask(context.Background(), "/repo", "TASK-1"); !errors.Is(err, errCommandTimeout) {
		t.Fatalf("err=%v, want timeout", err)
	}
	if len(r.calls) != 1 || len(rec.sleeps) != 0 || len(rec.logs) != 0 {
		t.Fatalf("attempts=%d sleeps=%v logs=%v, want one attempt and no retry", len(r.calls), rec.sleeps, rec.logs)
	}
}

func TestExecBacklogBackoffDoublesUpToCap(t *testing.T) {
	r := &flakyRunner{timeouts: 10}
	rec := &retryRecorder{}
	b := rec.backlog(r, 4)
	b.RetryBase = 30 * time.Second
	if _, err := b.Statuses(context.Background(), "/repo"); !errors.Is(err, errCommandTimeout) {
		t.Fatalf("err=%v, want timeout", err)
	}
	if fmt.Sprint(rec.sleeps) != "[30s 1m0s 1m0s 1m0s]" {
		t.Fatalf("backoff sleeps=%v, want [30s 1m0s 1m0s 1m0s]", rec.sleeps)
	}
}

func TestExecBacklogDoesNotRetryNonTimeoutErrors(t *testing.T) {
	failure := errors.New("backlog task view TASK-1 --json: exit status 1: boom")
	r := &flakyRunner{err: failure}
	rec := &retryRecorder{}
	_, err := rec.backlog(r, 2).ViewTask(context.Background(), "/repo", "TASK-1")
	if err != failure {
		t.Fatalf("err=%v, want the unchanged runner error", err)
	}
	if len(r.calls) != 1 || len(rec.sleeps) != 0 || len(rec.logs) != 0 {
		t.Fatalf("attempts=%d sleeps=%v logs=%v, want one attempt and no retry", len(r.calls), rec.sleeps, rec.logs)
	}
}

func TestExecBacklogDoesNotRetryTimedOutWrites(t *testing.T) {
	r := &flakyRunner{timeouts: 10}
	rec := &retryRecorder{}
	b := rec.backlog(r, 2)
	if _, err := b.CreateTask(context.Background(), "/repo", CreateTaskInput{Title: "title", Description: "desc"}); !errors.Is(err, errCommandTimeout) {
		t.Fatalf("CreateTask err=%v, want timeout", err)
	}
	if err := b.Push(context.Background(), "/repo", "main"); !errors.Is(err, errCommandTimeout) {
		t.Fatalf("Push err=%v, want timeout", err)
	}
	want := []string{"backlog task create --plain -d desc -- title", "git -C /repo push origin main"}
	if strings.Join(r.calls, "\n") != strings.Join(want, "\n") || len(rec.sleeps) != 0 || len(rec.logs) != 0 {
		t.Fatalf("calls=%v sleeps=%v logs=%v, want one attempt per write and no retry", r.calls, rec.sleeps, rec.logs)
	}
}

func TestExecBacklogParentCancelStopsRetries(t *testing.T) {
	t.Run("cancelled during the call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		r := &flakyRunner{timeouts: 10, onRun: cancel}
		rec := &retryRecorder{}
		if _, err := rec.backlog(r, 2).ViewTask(ctx, "/repo", "TASK-1"); !errors.Is(err, errCommandTimeout) {
			t.Fatalf("err=%v, want the call's timeout error", err)
		}
		if len(r.calls) != 1 || len(rec.sleeps) != 0 || len(rec.logs) != 0 {
			t.Fatalf("attempts=%d sleeps=%v logs=%v, want no retry after parent cancel", len(r.calls), rec.sleeps, rec.logs)
		}
	})
	t.Run("cancelled during backoff", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		r := &flakyRunner{timeouts: 10}
		b := ExecBacklog{Runner: r, Retries: 2, Sleep: func(ctx context.Context, _ time.Duration) error {
			cancel()
			// The real sleeper must return as soon as the context is done.
			return sleepContext(ctx, time.Hour)
		}}
		_, err := b.ViewTask(ctx, "/repo", "TASK-1")
		if !errors.Is(err, context.Canceled) || !errors.Is(err, errCommandTimeout) {
			t.Fatalf("err=%v, want timeout and context.Canceled", err)
		}
		if len(r.calls) != 1 {
			t.Fatalf("attempts=%d, want 1 after cancel during backoff", len(r.calls))
		}
	})
}

func TestOSCommandRunnerMarksOnlyItsOwnTimeout(t *testing.T) {
	_, err := OSCommandRunner{Timeout: 20 * time.Millisecond}.Run(context.Background(), "", "sleep", []string{"5"}, nil)
	if !errors.Is(err, errCommandTimeout) || err.Error() != "sleep 5: timed out after 20ms" {
		t.Fatalf("per-call timeout err=%v, want typed %q", err, "sleep 5: timed out after 20ms")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = OSCommandRunner{Timeout: time.Hour}.Run(ctx, "", "sleep", []string{"5"}, nil)
	if errors.Is(err, errCommandTimeout) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("parent deadline err=%v, want context.DeadlineExceeded and no per-call timeout", err)
	}
}
