package git

import "testing"

func TestParseWorktreeListTracksLockedReference(t *testing.T) {
	t.Parallel()

	entries := parseWorktreeList("worktree C:\\worktrees\\locked\nHEAD abc123\nbranch refs/heads/feature/locked\nlocked\n")
	if len(entries) != 1 {
		t.Fatalf("entries = %#v", entries)
	}
	if !entries[0].Locked {
		t.Fatalf("entry = %#v, want locked reference", entries[0])
	}
}
