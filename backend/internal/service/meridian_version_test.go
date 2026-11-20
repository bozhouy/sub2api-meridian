package service

import "testing"

func TestMeridianVersionComparison(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		want            int
	}{
		{"0.2.23-meridian.2", "v0.2.23-meridian.2", 0},
		{"0.2.23-meridian.1", "0.2.23-meridian.2", -1},
		{"0.2.23-meridian.10", "0.2.23-meridian.2", 1},
		{"0.2.23-meridian.10", "0.2.24-meridian.1", -1},
		{"0.2.23-hessian.1", "0.2.23-meridian.2", -1},
	} {
		if got := compareVersions(tc.current, tc.latest); got != tc.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tc.current, tc.latest, got, tc.want)
		}
	}
}
