package cli

import (
	"fmt"
	"testing"
)

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		want  semver
	}{
		{"v0.5.5", semver{0, 5, 5}},
		{"0.5.5", semver{0, 5, 5}},
		{"v1.2.3-28-gb603f04-dirty", semver{1, 2, 3}},
		{"v0.6.0-rc1", semver{0, 6, 0}},
		{"dev", semver{0, 0, 0}},
		{"v2.0.0", semver{2, 0, 0}},
		{"1.0", semver{1, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseSemver(tt.input)
			if got != tt.want {
				t.Errorf("parseSemver(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSemverGreaterThan(t *testing.T) {
	tests := []struct {
		a, b semver
		want bool
	}{
		{semver{0, 6, 0}, semver{0, 5, 5}, true},
		{semver{1, 0, 0}, semver{0, 9, 9}, true},
		{semver{0, 5, 6}, semver{0, 5, 5}, true},
		{semver{1, 0, 0}, semver{1, 0, 0}, false},
		{semver{0, 5, 5}, semver{0, 6, 0}, false},
		{semver{0, 0, 0}, semver{0, 5, 5}, false},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%d.%d.%d>%d.%d.%d", tt.a.major, tt.a.minor, tt.a.patch, tt.b.major, tt.b.minor, tt.b.patch)
		t.Run(name, func(t *testing.T) {
			got := tt.a.greaterThan(tt.b)
			if got != tt.want {
				t.Errorf("%s: got %v, want %v", name, got, tt.want)
			}
		})
	}
}
