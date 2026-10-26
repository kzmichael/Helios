package kafkax

import (
	"testing"
	"time"
)

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	if Backoff(1) >= Backoff(4) {
		t.Fatal("backoff should grow with attempts")
	}
	if got := Backoff(50); got > 14*time.Second {
		t.Fatalf("backoff %v exceeds cap plus jitter", got)
	}
}

func TestBackoffJitterStaysWithinBound(t *testing.T) {
	for i := 0; i < 200; i++ {
		d := Backoff(1)
		if d < 500*time.Millisecond || d > 650*time.Millisecond {
			t.Fatalf("attempt 1 backoff %v outside [500ms, 650ms]", d)
		}
	}
}
