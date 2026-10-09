package application

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCockpitSerialization(t *testing.T) {
	t.Parallel()

	// 1. CockpitRuntimeSnapshot
	t.Run("CockpitRuntimeSnapshot", func(t *testing.T) {
		snap := CockpitRuntimeSnapshot{
			SchemaVersion: CockpitBridgeSchemaVersion,
			Status:        "ready",
			Environment:   "local",
			Provider:      "disabled",
		}
		data, err := json.Marshal(snap)
		if err != nil {
			t.Fatal(err)
		}
		var decoded CockpitRuntimeSnapshot
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded != snap {
			t.Fatalf("mismatch: got %#v, want %#v", decoded, snap)
		}
	})

	// 2. CockpitWorkspaceRequest
	t.Run("CockpitWorkspaceRequest", func(t *testing.T) {
		req := CockpitWorkspaceRequest{
			SchemaVersion: CockpitBridgeSchemaVersion,
			ProjectID:     "project-123",
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		var decoded CockpitWorkspaceRequest
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded != req {
			t.Fatalf("mismatch: got %#v, want %#v", decoded, req)
		}
	})

	// 3. CockpitWorkspaceSnapshot with projects and worktrees
	t.Run("CockpitWorkspaceSnapshot", func(t *testing.T) {
		now := time.Now().Truncate(time.Second).UTC()
		snap := CockpitWorkspaceSnapshot{
			SchemaVersion: CockpitBridgeSchemaVersion,
			Projects: []CockpitProject{
				{
					ID:            "project-1",
					Name:          "Project One",
					DefaultBranch: "main",
					Status:        "active",
					CreatedAt:     now,
					UpdatedAt:     now,
					SchemaVersion: CockpitBridgeSchemaVersion,
				},
			},
			Worktrees: []CockpitWorktree{
				{
					ID:              "wt-1",
					ProjectID:       "project-1",
					Branch:          "feature",
					Revision:        "rev-1",
					Status:          "clean",
					Dirty:           false,
					ActiveReference: "HEAD",
					CreatedAt:       now,
					UpdatedAt:       now,
				},
			},
			VaultNoteCount:   15,
			WatcherState:     "running",
			WatcherErrorCode: "",
			RetrievalVersion: "cortexos.retrieval.v1",
			RetrievalState:   "indexed",
		}
		data, err := json.Marshal(snap)
		if err != nil {
			t.Fatal(err)
		}
		var decoded CockpitWorkspaceSnapshot
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.SchemaVersion != snap.SchemaVersion ||
			len(decoded.Projects) != 1 ||
			len(decoded.Worktrees) != 1 ||
			decoded.VaultNoteCount != 15 ||
			decoded.WatcherState != "running" ||
			decoded.RetrievalVersion != "cortexos.retrieval.v1" ||
			decoded.RetrievalState != "indexed" {
			t.Fatalf("mismatch in decoded snapshot: %#v", decoded)
		}
		if decoded.Projects[0].ID != "project-1" || !decoded.Projects[0].CreatedAt.Equal(now) {
			t.Fatalf("project mismatch: %#v", decoded.Projects[0])
		}
		if decoded.Worktrees[0].ID != "wt-1" || decoded.Worktrees[0].Dirty != false {
			t.Fatalf("worktree mismatch: %#v", decoded.Worktrees[0])
		}
	})

	// 4. CockpitProjectRequest
	t.Run("CockpitProjectRequest", func(t *testing.T) {
		req := CockpitProjectRequest{
			SchemaVersion:  CockpitBridgeSchemaVersion,
			ID:             "p1",
			Name:           "Project 1",
			RepositoryRoot: "/repo",
			VaultRoot:      "/vault",
			DefaultBranch:  "main",
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		var decoded CockpitProjectRequest
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded != req {
			t.Fatalf("mismatch: got %#v, want %#v", decoded, req)
		}
	})

	// 5. CockpitQueryRequest and CockpitQueryResult
	t.Run("CockpitQuery", func(t *testing.T) {
		req := CockpitQueryRequest{
			SchemaVersion: CockpitBridgeSchemaVersion,
			ProjectID:     "p1",
			Query:         "search query",
			Limit:         25,
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		var decodedReq CockpitQueryRequest
		if err := json.Unmarshal(data, &decodedReq); err != nil {
			t.Fatal(err)
		}
		if decodedReq != req {
			t.Fatalf("mismatch: got %#v, want %#v", decodedReq, req)
		}

		res := CockpitQueryResult{
			ID:            "res-1",
			NoteID:        "note-1",
			ProjectID:     "p1",
			RelativePath:  "docs/note.md",
			SourceHash:    "hash123",
			Attribution:   "author",
			IndexedAt:     time.Now().UTC().Format(time.RFC3339),
			Excerpt:       "test excerpt",
			SchemaVersion: CockpitBridgeSchemaVersion,
		}
		resData, err := json.Marshal(res)
		if err != nil {
			t.Fatal(err)
		}
		var decodedRes CockpitQueryResult
		if err := json.Unmarshal(resData, &decodedRes); err != nil {
			t.Fatal(err)
		}
		if decodedRes != res {
			t.Fatalf("mismatch: got %#v, want %#v", decodedRes, res)
		}
	})

	// 6. CockpitMutationRequest
	t.Run("CockpitMutationRequest", func(t *testing.T) {
		req := CockpitMutationRequest{
			SchemaVersion: CockpitBridgeSchemaVersion,
			ProjectID:     "p1",
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		var decoded CockpitMutationRequest
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded != req {
			t.Fatalf("mismatch: got %#v, want %#v", decoded, req)
		}
	})

	// 7. CockpitBridgeError
	t.Run("CockpitBridgeError", func(t *testing.T) {
		bErr := CockpitBridgeError{
			Code:    "cockpit.unsupported_version",
			Message: "cockpit bridge contract version is unsupported",
		}
		data, err := json.Marshal(bErr)
		if err != nil {
			t.Fatal(err)
		}
		var decoded CockpitBridgeError
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded != bErr {
			t.Fatalf("mismatch: got %#v, want %#v", decoded, bErr)
		}
		if decoded.Error() != "cockpit.unsupported_version: cockpit bridge contract version is unsupported" {
			t.Fatalf("Error() = %q", decoded.Error())
		}
	})
}
