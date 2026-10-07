package harness

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var _ EngineAdapter = (*NativeAdapter)(nil)

type fakeGitRunner struct {
	runFunc func(ctx context.Context, args ...string) ([]byte, []byte, error)
	calls   [][]string
}

func (f *fakeGitRunner) Run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	f.calls = append(f.calls, args)
	if f.runFunc != nil {
		return f.runFunc(ctx, args...)
	}
	return nil, nil, nil
}

type fakeValidationPort struct {
	runFunc func(ctx context.Context, op NativeOperationContext, req NativeValidationRequest) (NativeValidationResult, error)
	calls   []NativeValidationRequest
}

func (f *fakeValidationPort) Run(ctx context.Context, op NativeOperationContext, req NativeValidationRequest) (NativeValidationResult, error) {
	f.calls = append(f.calls, req)
	if f.runFunc != nil {
		return f.runFunc(ctx, op, req)
	}
	return NativeValidationResult{
		Kind:     req.Kind,
		Passed:   true,
		ExitCode: 0,
		Stdout:   "ok",
	}, nil
}

func testEnvelope(worktreeRoot, toolName string, input any) ExecutionEnvelope {
	raw, _ := json.Marshal(input)
	return ExecutionEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-test-1",
		TaskID:          "task-test-1",
		WorktreeID:      "wt-test-1",
		ProjectID:       "proj-test-1",
		WorktreeRoot:    worktreeRoot,
		ToolName:        toolName,
		Input:           raw,
		PolicyDecision: PolicyDecision{
			Allowed: true,
			Effect:  EffectAllow,
			Reason:  "policy allowed for test",
		},
		Timeout: 30 * time.Second,
		TraceID: "trace-test-1",
	}
}

func TestNativeAdapterContractDescriptor(t *testing.T) {
	tempDir := t.TempDir()
	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: &fakeGitRunner{}},
		Validation:   &fakeValidationPort{},
		WorktreeRoot: tempDir,
	})
	if err != nil {
		t.Fatalf("NewNativeAdapter failed: %v", err)
	}

	if adapter.Kind() != ToolKindNative {
		t.Fatalf("Kind = %v, want %v", adapter.Kind(), ToolKindNative)
	}

	desc := adapter.Describe()
	if desc.Name != NativeAdapterName {
		t.Fatalf("Describe().Name = %q, want %q", desc.Name, NativeAdapterName)
	}
	if desc.Version != NativeAdapterVersion {
		t.Fatalf("Describe().Version = %q, want %q", desc.Version, NativeAdapterVersion)
	}
	if desc.Kind != ToolKindNative {
		t.Fatalf("Describe().Kind = %v, want %v", desc.Kind, ToolKindNative)
	}
	if desc.SchemaVersion != HarnessContractVersion {
		t.Fatalf("Describe().SchemaVersion = %q, want %q", desc.SchemaVersion, HarnessContractVersion)
	}

	caps := adapter.Capabilities()
	expectedCaps := []ToolCapability{
		CapabilityFileRead, CapabilityFileWrite, CapabilityFileEdit,
		CapabilityGit, CapabilitySearch, CapabilityTestRun, CapabilityLint, CapabilityBuild,
	}
	for _, expected := range expectedCaps {
		found := false
		for _, c := range caps {
			if c == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected capability: %s", expected)
		}
	}
}

func TestNativeAdapterRouterRegistration(t *testing.T) {
	tempDir := t.TempDir()
	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: &fakeGitRunner{}},
		Validation:   &fakeValidationPort{},
		WorktreeRoot: tempDir,
	})
	if err != nil {
		t.Fatalf("NewNativeAdapter failed: %v", err)
	}

	router := NewBasicRouter(DefaultRoutingPolicy())
	desc := ExtendedAdapterDescriptor{
		AdapterDescriptor: adapter.Describe(),
		Identity: AdapterIdentity{
			Name:          adapter.Describe().Name,
			InstanceID:    "native-test-inst",
			Kind:          adapter.Kind(),
			Class:         EngineClassNative,
			Version:       adapter.Describe().Version,
			SchemaVersion: adapter.Describe().SchemaVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	}

	if err := router.Register(adapter, desc); err != nil {
		t.Fatalf("Router.Register failed: %v", err)
	}

	selection, err := router.Find(context.Background(), []ToolCapability{CapabilityFileRead, CapabilityFileWrite})
	if err != nil {
		t.Fatalf("Router.Find failed: %v", err)
	}
	if selection.Adapter.Kind() != ToolKindNative {
		t.Fatalf("Selection kind = %v, want %v", selection.Adapter.Kind(), ToolKindNative)
	}
}

func TestNativeAdapterHealthChecks(t *testing.T) {
	tempDir := t.TempDir()
	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: &fakeGitRunner{}},
		Validation:   &fakeValidationPort{},
		WorktreeRoot: tempDir,
	})
	if err != nil {
		t.Fatalf("NewNativeAdapter failed: %v", err)
	}

	ctx := NewExecutionContext(context.Background(), "trace-health")
	if err := adapter.Health(ctx); err != nil {
		t.Fatalf("expected healthy adapter, got %v", err)
	}

	// Canceled context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := adapter.Health(NewExecutionContext(canceledCtx, "trace-cancel")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// Nil / unconfigured ports
	emptyAdapter := &NativeAdapter{}
	if err := emptyAdapter.Health(ctx); !errors.Is(err, ErrAdapterUnhealthy) {
		t.Fatalf("expected ErrAdapterUnhealthy for unconfigured adapter, got %v", err)
	}

	// Unavailable worktree root
	missingRootAdapter := &NativeAdapter{
		workspace:  OSWorkspacePort{},
		git:        OSGitPort{Runner: &fakeGitRunner{}},
		validation: &fakeValidationPort{},
		root:       filepath.Join(tempDir, "does-not-exist"),
	}
	if err := missingRootAdapter.Health(ctx); !errors.Is(err, ErrAdapterUnhealthy) {
		t.Fatalf("expected ErrAdapterUnhealthy for missing root, got %v", err)
	}
}

func TestNativeAdapterFilesystemOperations_TempWorktree(t *testing.T) {
	tempDir := t.TempDir()
	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: &fakeGitRunner{}},
		Validation:   &fakeValidationPort{},
		WorktreeRoot: tempDir,
	})
	if err != nil {
		t.Fatalf("NewNativeAdapter failed: %v", err)
	}
	ctx := NewExecutionContext(context.Background(), "trace-fs")

	// 1. Write file
	writeEnv := testEnvelope(tempDir, "file_write", map[string]string{
		"path":    "hello.txt",
		"content": "Hello, CortexOS native tool!",
	})
	writeRes, err := adapter.Execute(ctx, writeEnv)
	if err != nil {
		t.Fatalf("file_write failed: %v", err)
	}
	if writeRes.Status != ToolStatusSuccess {
		t.Fatalf("file_write status = %v, want success", writeRes.Status)
	}
	if len(writeRes.EvidenceIDs) == 0 {
		t.Fatal("file_write missing evidence IDs")
	}

	// Verify file was written to disk in tempDir
	diskContent, err := os.ReadFile(filepath.Join(tempDir, "hello.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile failed: %v", err)
	}
	if string(diskContent) != "Hello, CortexOS native tool!" {
		t.Fatalf("file content mismatch: got %q", string(diskContent))
	}

	// 2. Read file
	readEnv := testEnvelope(tempDir, "file_read", map[string]string{
		"path": "hello.txt",
	})
	readRes, err := adapter.Execute(ctx, readEnv)
	if err != nil {
		t.Fatalf("file_read failed: %v", err)
	}
	if readRes.Status != ToolStatusSuccess {
		t.Fatalf("file_read status = %v, want success", readRes.Status)
	}
	var readOut struct {
		Result struct {
			Content string `json:"content"`
			Path    string `json:"path"`
		} `json:"result"`
	}
	if err := json.Unmarshal(readRes.Output, &readOut); err != nil {
		t.Fatalf("unmarshal read output: %v", err)
	}
	if readOut.Result.Content != "Hello, CortexOS native tool!" {
		t.Fatalf("read content = %q, want %q", readOut.Result.Content, "Hello, CortexOS native tool!")
	}

	// 3. Edit file
	editEnv := testEnvelope(tempDir, "file_edit", map[string]string{
		"path": "hello.txt",
		"old":  "native tool",
		"new":  "deterministic adapter",
	})
	editRes, err := adapter.Execute(ctx, editEnv)
	if err != nil {
		t.Fatalf("file_edit failed: %v", err)
	}
	if editRes.Status != ToolStatusSuccess {
		t.Fatalf("file_edit status = %v, want success", editRes.Status)
	}

	// Verify on-disk after edit
	diskContentEdited, err := os.ReadFile(filepath.Join(tempDir, "hello.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile after edit: %v", err)
	}
	if string(diskContentEdited) != "Hello, CortexOS deterministic adapter!" {
		t.Fatalf("edited content mismatch: got %q", string(diskContentEdited))
	}

	// 4. Search
	searchEnv := testEnvelope(tempDir, "search", map[string]string{
		"query": "deterministic",
	})
	searchRes, err := adapter.Execute(ctx, searchEnv)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if searchRes.Status != ToolStatusSuccess {
		t.Fatalf("search status = %v, want success", searchRes.Status)
	}
	var searchOut struct {
		Result struct {
			Matches []NativeSearchMatch `json:"matches"`
		} `json:"result"`
	}
	if err := json.Unmarshal(searchRes.Output, &searchOut); err != nil {
		t.Fatalf("unmarshal search output: %v", err)
	}
	if len(searchOut.Result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(searchOut.Result.Matches))
	}
	if searchOut.Result.Matches[0].Path != "hello.txt" {
		t.Fatalf("match path = %q, want hello.txt", searchOut.Result.Matches[0].Path)
	}
}

func TestNativeAdapterEditRequiresUniqueMatch(t *testing.T) {
	tempDir := t.TempDir()
	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: &fakeGitRunner{}},
		Validation:   &fakeValidationPort{},
		WorktreeRoot: tempDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := NewExecutionContext(context.Background(), "trace-edit-unique")

	filePath := filepath.Join(tempDir, "multi.txt")
	if err := os.WriteFile(filePath, []byte("repeat repeat repeat"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Attempt edit on ambiguous pattern
	editEnv := testEnvelope(tempDir, "file_edit", map[string]string{
		"path": "multi.txt",
		"old":  "repeat",
		"new":  "once",
	})
	_, err = adapter.Execute(ctx, editEnv)
	if err == nil {
		t.Fatal("expected error on ambiguous edit match")
	}
	if !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("expected ErrInvalidContract, got %v", err)
	}
}

func TestNativeAdapterWorktreeBoundaryContainment(t *testing.T) {
	tempDir := t.TempDir()
	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: &fakeGitRunner{}},
		Validation:   &fakeValidationPort{},
		WorktreeRoot: tempDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := NewExecutionContext(context.Background(), "trace-traversal")

	// 1. Read outside boundary
	readEscapeEnv := testEnvelope(tempDir, "file_read", map[string]string{
		"path": "../secret.txt",
	})
	_, err = adapter.Execute(ctx, readEscapeEnv)
	if err == nil {
		t.Fatal("expected traversal read to be rejected")
	}

	// 2. Write outside boundary
	writeEscapeEnv := testEnvelope(tempDir, "file_write", map[string]string{
		"path":    "../../escape.txt",
		"content": "escape",
	})
	_, err = adapter.Execute(ctx, writeEscapeEnv)
	if err == nil {
		t.Fatal("expected traversal write to be rejected")
	}

	// 3. Absolute path outside boundary
	outsidePath := filepath.Join(os.TempDir(), "arbitrary-file.txt")
	absWriteEnv := testEnvelope(tempDir, "file_write", map[string]string{
		"path":    outsidePath,
		"content": "escape",
	})
	_, err = adapter.Execute(ctx, absWriteEnv)
	if err == nil {
		t.Fatal("expected absolute path write to be rejected")
	}
}

func TestNativeAdapterGitOperations(t *testing.T) {
	tempDir := t.TempDir()
	runner := &fakeGitRunner{
		runFunc: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
			cmdStr := strings.Join(args, " ")
			if strings.Contains(cmdStr, "status") {
				return []byte("## main\n M file1.go\n?? file2.go\n"), nil, nil
			}
			if strings.Contains(cmdStr, "diff") {
				return []byte("diff --git a/file1.go b/file1.go\n--- a/file1.go\n+++ b/file1.go\n@@ -1 +1 @@\n-old\n+new\n"), nil, nil
			}
			return nil, nil, errors.New("unsupported git command")
		},
	}

	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: runner},
		Validation:   &fakeValidationPort{},
		WorktreeRoot: tempDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := NewExecutionContext(context.Background(), "trace-git")

	// 1. Git Status
	statusEnv := testEnvelope(tempDir, "git_status", map[string]any{})
	statusRes, err := adapter.Execute(ctx, statusEnv)
	if err != nil {
		t.Fatalf("git_status failed: %v", err)
	}
	if statusRes.Status != ToolStatusSuccess {
		t.Fatalf("git_status status = %v, want success", statusRes.Status)
	}

	var statusOut struct {
		Result NativeGitStatus `json:"result"`
	}
	if err := json.Unmarshal(statusRes.Output, &statusOut); err != nil {
		t.Fatalf("unmarshal git status: %v", err)
	}
	if statusOut.Result.Branch != "main" {
		t.Fatalf("branch = %q, want main", statusOut.Result.Branch)
	}
	if !statusOut.Result.Dirty {
		t.Fatal("expected dirty git status")
	}
	if len(statusOut.Result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(statusOut.Result.Entries))
	}

	// 2. Git Diff
	diffEnv := testEnvelope(tempDir, "git_diff", map[string]string{
		"path": "file1.go",
	})
	diffRes, err := adapter.Execute(ctx, diffEnv)
	if err != nil {
		t.Fatalf("git_diff failed: %v", err)
	}
	if diffRes.Status != ToolStatusSuccess {
		t.Fatalf("git_diff status = %v, want success", diffRes.Status)
	}

	var diffOut struct {
		Result NativeGitDiff `json:"result"`
	}
	if err := json.Unmarshal(diffRes.Output, &diffOut); err != nil {
		t.Fatalf("unmarshal git diff: %v", err)
	}
	if !strings.Contains(diffOut.Result.Patch, "+new") {
		t.Fatalf("diff patch does not contain expected change: %q", diffOut.Result.Patch)
	}
}

func TestNativeAdapterValidationOperations(t *testing.T) {
	tempDir := t.TempDir()
	valPort := &fakeValidationPort{
		runFunc: func(ctx context.Context, op NativeOperationContext, req NativeValidationRequest) (NativeValidationResult, error) {
			return NativeValidationResult{
				Kind:     req.Kind,
				Passed:   true,
				ExitCode: 0,
				Stdout:   "PASS\nok\n",
			}, nil
		},
	}

	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: &fakeGitRunner{}},
		Validation:   valPort,
		WorktreeRoot: tempDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := NewExecutionContext(context.Background(), "trace-val")

	testKinds := []string{"test_run", "lint", "build"}
	for _, kind := range testKinds {
		env := testEnvelope(tempDir, kind, map[string]any{})
		res, err := adapter.Execute(ctx, env)
		if err != nil {
			t.Fatalf("validation %s failed: %v", kind, err)
		}
		if res.Status != ToolStatusSuccess {
			t.Fatalf("validation %s status = %v, want success", kind, res.Status)
		}
	}

	if len(valPort.calls) != 3 {
		t.Fatalf("expected 3 validation calls, got %d", len(valPort.calls))
	}
}

func TestNativeAdapterUnsupportedOperationsNoFallbackShell(t *testing.T) {
	tempDir := t.TempDir()
	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: &fakeGitRunner{}},
		Validation:   &fakeValidationPort{},
		WorktreeRoot: tempDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := NewExecutionContext(context.Background(), "trace-unsupported")

	unsupportedOps := []string{
		"shell",
		"bash",
		"sh",
		"exec",
		"python",
		"arbitrary_cmd",
		"git_commit",
		"git_push",
		"rm",
	}

	for _, op := range unsupportedOps {
		t.Run(op, func(t *testing.T) {
			env := testEnvelope(tempDir, op, map[string]string{"cmd": "echo pwned"})
			res, err := adapter.Execute(ctx, env)
			if err == nil {
				t.Fatalf("expected unsupported error for operation %q", op)
			}
			if !errors.Is(err, ErrNativeUnsupportedOperation) {
				t.Fatalf("expected ErrNativeUnsupportedOperation for %q, got %v", op, err)
			}
			if res.Status != ToolStatusFailed && res.Status != ToolStatusAdapterError {
				t.Fatalf("expected failed/adapter_error status, got %v", res.Status)
			}
		})
	}
}

func TestNativeAdapterPolicyEnforcement(t *testing.T) {
	tempDir := t.TempDir()
	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: &fakeGitRunner{}},
		Validation:   &fakeValidationPort{},
		WorktreeRoot: tempDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := NewExecutionContext(context.Background(), "trace-policy")

	deniedEnv := testEnvelope(tempDir, "file_read", map[string]string{"path": "file.txt"})
	deniedEnv.PolicyDecision = PolicyDecision{
		Allowed: false,
		Effect:  EffectDeny,
		Reason:  "policy denied this execution",
	}

	_, err = adapter.Execute(ctx, deniedEnv)
	if err == nil {
		t.Fatal("expected execution with denied policy to fail")
	}
	if !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("expected ErrPolicyDenied, got %v", err)
	}
}

func TestNativeAdapterRedactionAndBoundedness(t *testing.T) {
	tempDir := t.TempDir()
	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: &fakeGitRunner{}},
		Validation:   &fakeValidationPort{},
		WorktreeRoot: tempDir,
		MaxBytes:     100, // Small limit for testing boundedness
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := NewExecutionContext(context.Background(), "trace-bounds")

	// 1. Exceed maxBytes on write
	bigContent := strings.Repeat("A", 200)
	bigWriteEnv := testEnvelope(tempDir, "file_write", map[string]string{
		"path":    "big.txt",
		"content": bigContent,
	})
	_, err = adapter.Execute(ctx, bigWriteEnv)
	if err == nil {
		t.Fatal("expected error when writing content exceeding maxBytes")
	}
	if !errors.Is(err, ErrAdapterFailure) {
		t.Fatalf("expected ErrAdapterFailure, got %v", err)
	}

	// 2. Secret Redaction in output
	secretContent := "api_key: sk-123456789012345678901234"
	secretWriteEnv := testEnvelope(tempDir, "file_write", map[string]string{
		"path":    "secret.txt",
		"content": secretContent,
	})
	res, err := adapter.Execute(ctx, secretWriteEnv)
	if err != nil {
		t.Fatalf("write secret file failed: %v", err)
	}
	if strings.Contains(string(res.Output), "sk-123456789012345678901234") {
		t.Fatalf("secret was not redacted in output: %s", string(res.Output))
	}
}

func TestNativeAdapterBrokerIntegration(t *testing.T) {

	tempDir := t.TempDir()
	policy := mustPolicy(Policy{
		Rules: []Rule{
			{ID: "r1", Action: "file_write", Resource: "*", Effect: EffectAllow},
			{ID: "r2", Action: "file_read", Resource: "*", Effect: EffectAllow},
			{ID: "r3", Action: "search", Resource: "*", Effect: EffectAllow},
		},
	})
	broker := NewBroker(policy)

	adapter, err := NewNativeAdapter(NativeAdapterConfig{
		Workspace:    OSWorkspacePort{},
		Git:          OSGitPort{Runner: &fakeGitRunner{}},
		Validation:   &fakeValidationPort{},
		WorktreeRoot: tempDir,
	})
	if err != nil {
		t.Fatalf("NewNativeAdapter failed: %v", err)
	}

	if err := broker.RegisterEngine(adapter); err != nil {
		t.Fatalf("broker.RegisterEngine failed: %v", err)
	}

	// Register tools that map to native capabilities
	toolWrite := ToolDefinition{
		Name:         "native_write",
		Kind:         ToolKindNative,
		Description:  "Write file",
		Capabilities: []ToolCapability{CapabilityFileWrite},
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0",
	}
	if err := broker.Register(toolWrite); err != nil {
		t.Fatalf("broker.Register native_write: %v", err)
	}

	toolRead := ToolDefinition{
		Name:         "native_read",
		Kind:         ToolKindNative,
		Description:  "Read file",
		Capabilities: []ToolCapability{CapabilityFileRead},
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0",
	}
	if err := broker.Register(toolRead); err != nil {
		t.Fatalf("broker.Register native_read: %v", err)
	}

	ctx := NewExecutionContext(context.Background(), "trace-broker-native")

	// 1. Execute Write through Broker
	writeInput, _ := json.Marshal(map[string]string{
		"path":    "integrated.txt",
		"content": "Broker to Native Adapter test!",
	})
	writeReq := ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-broker-1",
		TaskID:          "task-broker-1",
		WorktreeID:      tempDir,
		ToolName:        "native_write",
		Action:          string(CapabilityFileWrite),
		Input:           writeInput,
		Timeout:         10 * time.Second,
		TraceID:         "trace-broker-native",
	}

	writeRes, err := broker.Execute(ctx, writeReq)
	if err != nil {
		t.Fatalf("broker.Execute write failed: %v", err)
	}
	if writeRes.Status != ToolStatusSuccess {
		t.Fatalf("broker write status = %v, want success", writeRes.Status)
	}

	// 2. Execute Read through Broker
	readInput, _ := json.Marshal(map[string]string{
		"path": "integrated.txt",
	})
	readReq := ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-broker-2",
		TaskID:          "task-broker-2",
		WorktreeID:      tempDir,
		ToolName:        "native_read",
		Action:          string(CapabilityFileRead),
		Input:           readInput,
		Timeout:         10 * time.Second,
		TraceID:         "trace-broker-native",
	}

	readRes, err := broker.Execute(ctx, readReq)
	if err != nil {
		t.Fatalf("broker.Execute read failed: %v", err)
	}
	if readRes.Status != ToolStatusSuccess {
		t.Fatalf("broker read status = %v, want success", readRes.Status)
	}

	// Verify audit log has recorded decisions and executions
	auditEvents := broker.AuditEvents()
	if len(auditEvents) == 0 {
		t.Fatal("expected audit events in broker log")
	}
}
