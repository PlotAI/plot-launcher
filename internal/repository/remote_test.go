package repository

import "testing"

func TestSameRepository(t *testing.T) {
	tests := []struct {
		name   string
		first  string
		second string
		want   bool
	}{
		{
			name:   "https",
			first:  "https://github.com/PlotAI/plot-launcher.git",
			second: "https://github.com/PlotAI/plot-launcher.git",
			want:   true,
		},
		{
			name:   "https without suffix",
			first:  "https://github.com/PlotAI/plot-launcher",
			second: "https://github.com/PlotAI/plot-launcher.git",
			want:   true,
		},
		{
			name:   "scp form",
			first:  "git@github.com:PlotAI/plot-launcher.git",
			second: "https://github.com/PlotAI/plot-launcher.git",
			want:   true,
		},
		{
			name:   "ssh form",
			first:  "ssh://git@github.com/PlotAI/plot-launcher.git",
			second: "https://github.com/PlotAI/plot-launcher.git",
			want:   true,
		},
		{
			name:   "case insensitive",
			first:  "git@github.com:plotai/Plot-Launcher.git",
			second: "https://github.com/PlotAI/plot-launcher.git",
			want:   true,
		},
		{
			name:   "transferred repository",
			first:  "https://github.com/OldOwner/plot-launcher.git",
			second: "https://github.com/NewOwner/plot-launcher.git",
			want:   false,
		},
		{
			name:   "renamed repository",
			first:  "https://github.com/PlotAI/plot-launcher.git",
			second: "https://github.com/PlotAI/plot-studio.git",
			want:   false,
		},
		{
			name:   "another host",
			first:  "https://gitlab.com/PlotAI/plot-launcher.git",
			second: "https://github.com/PlotAI/plot-launcher.git",
			want:   false,
		},
		{
			name:   "local path",
			first:  "file:///tmp/origin.git",
			second: "file:///tmp/origin.git",
			want:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sameRepository(test.first, test.second); got != test.want {
				t.Fatalf("sameRepository(%q, %q) = %t, want %t", test.first, test.second, got, test.want)
			}
		})
	}
}

func TestMovedRemote(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		requested string
		want      string
	}{
		{
			name:      "keeps https",
			current:   "https://github.com/OldOwner/plot-launcher.git",
			requested: "https://github.com/NewOwner/plot-launcher.git",
			want:      "https://github.com/NewOwner/plot-launcher.git",
		},
		{
			name:      "keeps ssh",
			current:   "git@github.com:OldOwner/plot-launcher.git",
			requested: "https://github.com/NewOwner/plot-launcher.git",
			want:      "git@github.com:NewOwner/plot-launcher.git",
		},
		{
			name:      "unknown current uses the request",
			current:   "file:///tmp/origin.git",
			requested: "https://github.com/NewOwner/plot-launcher.git",
			want:      "https://github.com/NewOwner/plot-launcher.git",
		},
		{
			name:      "unknown request uses the request",
			current:   "git@github.com:OldOwner/plot-launcher.git",
			requested: "file:///tmp/moved.git",
			want:      "file:///tmp/moved.git",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := movedRemote(test.current, test.requested); got != test.want {
				t.Fatalf("movedRemote(%q, %q) = %q, want %q", test.current, test.requested, got, test.want)
			}
		})
	}
}
