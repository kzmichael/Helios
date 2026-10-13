// Package profiles loads behaviour presets that make simulated fleets look like
// a real mix of vehicles instead of identical robots — see
// docs/07_REPOSITORY_BLUEPRINT.md §3.7 for why this matters.
package profiles

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed *.yaml
var files embed.FS

type SpeedRange struct {
	Min    float64 `yaml:"min"`
	Max    float64 `yaml:"max"`
	Cruise float64 `yaml:"cruise"`
}

type Profile struct {
	Name                    string     `yaml:"name"`
	EmissionHz              float64    `yaml:"emission_hz"`
	SpeedKph                SpeedRange `yaml:"speed_kph"`
	IdleProbability         float64    `yaml:"idle_probability"`
	IgnitionOffProbability  float64    `yaml:"ignition_off_probability"`
	HeadingChangeDegPerTick float64    `yaml:"heading_change_deg_per_tick"`
	GPSNoiseM               float64    `yaml:"gps_noise_m"`
}

func Load(name string) (Profile, error) {
	data, err := files.ReadFile(name + ".yaml")
	if err != nil {
		return Profile{}, fmt.Errorf("unknown profile %q: %w", name, err)
	}
	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("invalid profile %q: %w", name, err)
	}
	return p, nil
}

// Names lists the profiles device-sim ships with. A fleet mixes these rather
// than running one profile for every device, because uniform simulated devices
// hide the load patterns real, mixed fleets actually produce.
var Names = []string{"commuter", "long-haul", "idle", "erratic"}
