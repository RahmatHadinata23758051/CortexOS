package platform

import (
	"testing"
	"time"
)

func TestSmokeQuitDelay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		ok   bool
		want time.Duration
	}{
		{raw: "", ok: false},
		{raw: "invalid", ok: false},
		{raw: "-1", ok: false},
		{raw: "0", ok: true, want: 0},
		{raw: "1500", ok: true, want: 1500 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, ok := smokeQuitDelay(tt.raw)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("smokeQuitDelay(%q) = (%s, %t), want (%s, %t)", tt.raw, got, ok, tt.want, tt.ok)
			}
		})
	}
}
