package bridge

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

type fakePiBrainRunner struct {
	stdout string
}

func (f *fakePiBrainRunner) Run(ctx context.Context, cmd string, args []string, env []string, workDir string,
	stdoutLimit, stderrLimit int64, gracePeriod time.Duration) (harness.SandboxResult, error) {
	return harness.SandboxResult{Stdout: []byte(f.stdout), ExitCode: 0}, nil
}

func TestPiBrainAdapter_DispatchAndStream(t *testing.T) {
	stream := `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"{\"intent\":\"inquiry\",\"reply\":\"Halo!\"}"}]}}`
	runner := &fakePiBrainRunner{stdout: stream}
	piAdapter, err := harness.NewPiCLIAdapter(harness.PiCLIAdapterConfig{
		ExecutablePath: "pi",
		WorktreeRoot:   t.TempDir(),
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("NewPiCLIAdapter: %v", err)
	}

	adapter, err := NewPiBrainAdapter(piAdapter, "system role", 5*time.Second)
	if err != nil {
		t.Fatalf("NewPiBrainAdapter: %v", err)
	}
	if adapter.Name() != "pi-cli-brain" {
		t.Fatalf("unexpected adapter name: %s", adapter.Name())
	}

	ctx := context.Background()
	res, err := adapter.Dispatch(ctx, application.BrainDispatchRequest{
		TaskID:     "task-pi-1",
		UserPrompt: "halo",
	})
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}
	if res.State != "completed" || !strings.Contains(res.Output, "inquiry") {
		t.Fatalf("unexpected dispatch result: %+v", res)
	}

	var chunks []application.StreamChunk
	streamRes, err := adapter.DispatchStream(ctx, application.BrainDispatchRequest{
		TaskID:     "task-pi-2",
		UserPrompt: "halo",
	}, func(c application.StreamChunk) {
		chunks = append(chunks, c)
	})
	if err != nil || streamRes.State != "completed" || len(chunks) != 1 {
		t.Fatalf("DispatchStream failed: res=%+v chunks=%d err=%v", streamRes, len(chunks), err)
	}
}

func TestPiBrainAdapter_NilRunnerFails(t *testing.T) {
	_, err := NewPiBrainAdapter(nil, "role", time.Second)
	if err == nil {
		t.Fatalf("expected error for nil runner")
	}
}
