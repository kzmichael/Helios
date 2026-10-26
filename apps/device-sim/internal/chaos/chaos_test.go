package chaos

import (
	"testing"
	"time"
)

func TestDisabledByDefault(t *testing.T) {
	var c Config
	if c.Enabled() {
		t.Fatal("zero config must not inject faults")
	}
	inj := c.For(3)
	if inj.ShouldDrop() || inj.ShouldDuplicate() || inj.Skew() != 0 || inj.NextDisconnect() != 0 {
		t.Fatal("zero config injected a fault")
	}
}

func TestPacketLossRoughlyMatchesConfig(t *testing.T) {
	inj := Config{PacketLossPct: 25, Seed: 1}.For(0)
	dropped := 0
	for i := 0; i < 10000; i++ {
		if inj.ShouldDrop() {
			dropped++
		}
	}
	if dropped < 2200 || dropped > 2800 {
		t.Fatalf("expected ~25%% loss, dropped %d of 10000", dropped)
	}
}

func TestSkewAppliesToFractionAndAlternatesSign(t *testing.T) {
	c := Config{ClockSkew: 30 * time.Second, SkewFraction: 1, Seed: 1}
	if c.For(0).Skew() != 30*time.Second {
		t.Fatal("even index should skew forward")
	}
	if c.For(1).Skew() != -30*time.Second {
		t.Fatal("odd index should skew backward")
	}
	if (Config{ClockSkew: time.Second, SkewFraction: 0, Seed: 1}).For(0).Skew() != 0 {
		t.Fatal("fraction 0 must skew no devices")
	}
}

func TestSameSeedIsRepeatable(t *testing.T) {
	c := Config{PacketLossPct: 50, Seed: 42}
	a, b := c.For(5), c.For(5)
	for i := 0; i < 100; i++ {
		if a.ShouldDrop() != b.ShouldDrop() {
			t.Fatal("same seed and index must give the same sequence")
		}
	}
}

func TestDisconnectIntervalHasFloor(t *testing.T) {
	inj := Config{DisconnectMean: 10 * time.Millisecond, Seed: 1}.For(0)
	for i := 0; i < 100; i++ {
		if inj.NextDisconnect() < time.Second {
			t.Fatal("disconnect interval below one second")
		}
	}
}
