package backpressure

import (
	"errors"
	"testing"
	"time"
)

func TestEnterpriseIsNeverShed(t *testing.T) {
	for level := 0; level <= 4; level++ {
		for seq := uint64(0); seq < 8; seq++ {
			if Decide(level, "enterprise", seq) != Accept {
				t.Fatalf("enterprise was shed at level %d", level)
			}
		}
	}
}

func TestFreeIsShedBeforePro(t *testing.T) {
	// Level 1 samples Free only.
	if Decide(1, "pro", 1) != Accept {
		t.Fatal("pro must be untouched at level 1")
	}
	if Decide(1, "free", 1) != Sample {
		t.Fatal("free should be sampled at level 1")
	}
	// Level 2 rejects Free; Pro still untouched.
	if Decide(2, "free", 1) != Reject || Decide(2, "pro", 1) != Accept {
		t.Fatal("level 2 should reject free and leave pro alone")
	}
	// Level 3 samples Pro, level 4 rejects it.
	if Decide(3, "pro", 1) != Sample || Decide(4, "pro", 1) != Reject {
		t.Fatal("pro should be sampled at 3 and rejected at 4")
	}
}

func TestSamplingKeepsOneInN(t *testing.T) {
	kept := 0
	for seq := uint64(1); seq <= 100; seq++ {
		if Decide(1, "free", seq) == Accept {
			kept++
		}
	}
	if kept != 100/SampleEvery {
		t.Fatalf("expected %d kept, got %d", 100/SampleEvery, kept)
	}
}

func TestUnknownTierIsTreatedAsFree(t *testing.T) {
	if Decide(2, "mystery", 1) != Reject {
		t.Fatal("an unrecognised tier must be shed like free")
	}
}

func TestLevelForThresholds(t *testing.T) {
	cases := map[float64]int{0.5: 0, 0.99: 0, 1.0: 1, 1.3: 2, 1.6: 3, 2.5: 4}
	for p, want := range cases {
		if got := LevelFor(p); got != want {
			t.Errorf("pressure %.2f: got level %d want %d", p, got, want)
		}
	}
}

func TestBacklogDrivesPressure(t *testing.T) {
	s := New(100)
	dones := make([]func(error), 0, 100)
	for i := 0; i < 100; i++ {
		dones = append(dones, s.Begin())
	}
	if p := s.Pressure(time.Now()); p < 1.0 {
		t.Fatalf("100 in flight against capacity 100 should exceed the 80%% threshold, got %.2f", p)
	}
	for _, d := range dones {
		d(nil)
	}
	if p := s.Pressure(time.Now()); p >= 1.0 {
		t.Fatalf("pressure should fall once publishes complete, got %.2f", p)
	}
}

func TestErrorRateDrivesPressure(t *testing.T) {
	s := New(1000)
	for i := 0; i < 50; i++ {
		var err error
		if i%2 == 0 {
			err = errors.New("kafka down")
		}
		s.Begin()(err)
	}
	if p := s.Pressure(time.Now()); p < 1.0 {
		t.Fatalf("a 50%% error rate should exceed the 10%% threshold, got %.2f", p)
	}
}

func TestLevelEscalatesAtOnceAndRelaxesSlowly(t *testing.T) {
	s := New(10)
	now := time.Now()
	for i := 0; i < 10; i++ {
		s.Begin() // never completed: sustained backlog
	}
	if lvl := s.Update(now); lvl == 0 {
		t.Fatal("expected the level to escalate immediately")
	}
	high := s.Level()

	s.inflight.Store(0)
	if s.Update(now.Add(time.Second)); s.Level() != high {
		t.Fatal("level must not drop the instant pressure eases")
	}
	s.Update(now.Add(12 * time.Second))
	if s.Level() >= high {
		t.Fatal("level should relax after ten calm seconds")
	}
}
