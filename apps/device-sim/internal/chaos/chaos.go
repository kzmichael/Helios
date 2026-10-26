// Package chaos injects the transport and device faults real fleets produce:
// skewed device clocks, lost and duplicated points, and dropped connections.
// Everything is off unless configured, and seeded so a run can be repeated.
package chaos

import (
	"math"
	"math/rand"
	"time"
)

type Config struct {
	// ClockSkew is added to (or subtracted from) recorded_at for SkewFraction of devices.
	ClockSkew    time.Duration
	SkewFraction float64

	// PacketLossPct and DuplicatePct are percentages in [0, 100] applied per point.
	PacketLossPct float64
	DuplicatePct  float64

	// DisconnectMean is the mean time between forced stream disconnects. Zero disables.
	DisconnectMean time.Duration

	Seed int64
}

func (c Config) Enabled() bool {
	return c.ClockSkew != 0 || c.PacketLossPct > 0 || c.DuplicatePct > 0 || c.DisconnectMean > 0
}

// Injector holds per-device fault state. It is not safe for concurrent use; each
// simulated device owns one.
type Injector struct {
	cfg  Config
	rng  *rand.Rand
	skew time.Duration
}

// For returns the injector for the device at position index.
func (c Config) For(index int64) *Injector {
	rng := rand.New(rand.NewSource(c.Seed + index*7919))
	inj := &Injector{cfg: c, rng: rng}
	if c.ClockSkew != 0 && rng.Float64() < c.SkewFraction {
		inj.skew = c.ClockSkew
		if index%2 == 1 {
			inj.skew = -c.ClockSkew
		}
	}
	return inj
}

// Skew is the fixed clock offset this device suffers.
func (i *Injector) Skew() time.Duration { return i.skew }

// ShouldDrop reports whether the next point is lost in transit.
func (i *Injector) ShouldDrop() bool {
	return i.cfg.PacketLossPct > 0 && i.rng.Float64()*100 < i.cfg.PacketLossPct
}

// ShouldDuplicate reports whether the next point is delivered twice.
func (i *Injector) ShouldDuplicate() bool {
	return i.cfg.DuplicatePct > 0 && i.rng.Float64()*100 < i.cfg.DuplicatePct
}

// NextDisconnect returns how long until the next forced disconnect, or zero if
// disconnect injection is disabled. Intervals are exponentially distributed.
func (i *Injector) NextDisconnect() time.Duration {
	if i.cfg.DisconnectMean <= 0 {
		return 0
	}
	d := time.Duration(-math.Log(1-i.rng.Float64()) * float64(i.cfg.DisconnectMean))
	if d < time.Second {
		d = time.Second
	}
	return d
}
