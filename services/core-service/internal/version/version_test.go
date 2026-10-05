package version

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "plain semver", input: "1.2.0", want: "1.2.0"},
		{name: "lowercase v prefix", input: "v1.2.0", want: "1.2.0"},
		{name: "uppercase v prefix", input: "V1.2.0", want: "1.2.0"},
		{name: "zeroes", input: "0.0.0", want: "0.0.0"},
		{name: "large components", input: "10.20.30", want: "10.20.30"},
		{name: "leading zeroes preserved", input: "1.02.3", want: "1.02.3"},

		{name: "empty", input: "", wantErr: true},
		{name: "v only", input: "v", wantErr: true},
		{name: "missing patch", input: "1.2", wantErr: true},
		{name: "too many components", input: "1.2.3.4", wantErr: true},
		{name: "empty major", input: ".2.0", wantErr: true},
		{name: "empty minor", input: "1..0", wantErr: true},
		{name: "empty patch", input: "1.2.", wantErr: true},
		{name: "non numeric major", input: "x.2.0", wantErr: true},
		{name: "non numeric minor", input: "1.y.0", wantErr: true},
		{name: "non numeric patch", input: "1.2.z", wantErr: true},
		{name: "negative component", input: "1.-2.0", wantErr: true},
		{name: "prerelease rejected", input: "1.2.0-rc1", wantErr: true},
		{name: "build metadata rejected", input: "1.2.0+build5", wantErr: true},
		{name: "inner space rejected", input: "1. 2.0", wantErr: true},
		{name: "trailing newline rejected", input: "1.2.0\n", wantErr: true},
		{name: "leading space rejected", input: " 1.2.0", wantErr: true},
		{name: "too long", input: "1.2.3.4.5.6.7.8.9.10.11.12.13.14.15.16.17.18.19.20.21.22.23", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %q, want error", tt.input, got)
				}
				if Semver(tt.input) {
					t.Fatalf("Semver(%q) = true, want false", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if !Semver(tt.input) {
				t.Errorf("Semver(%q) = false, want true", tt.input)
			}
		})
	}
}
