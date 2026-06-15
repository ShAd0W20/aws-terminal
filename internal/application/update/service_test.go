package update

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "equal with v", a: "v1.2.3", b: "1.2.3", want: 0},
		{name: "patch older", a: "v1.2.3", b: "v1.2.4", want: -1},
		{name: "minor newer", a: "v1.3.0", b: "v1.2.9", want: 1},
		{name: "missing patch equal zero", a: "v1.2", b: "v1.2.0", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareVersions(tt.a, tt.b)
			if got < 0 {
				got = -1
			} else if got > 0 {
				got = 1
			}
			if got != tt.want {
				t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsDevelopmentVersion(t *testing.T) {
	for _, version := range []string{"", "dev", "DEV", "(devel)"} {
		if !IsDevelopmentVersion(version) {
			t.Fatalf("expected %q to be development", version)
		}
	}
	if IsDevelopmentVersion("v1.0.0") {
		t.Fatal("release version should not be development")
	}
}
