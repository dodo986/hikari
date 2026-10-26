package engine

import (
	"reflect"
	"testing"
)

func TestLegendaryInstallArgs(t *testing.T) {
	tests := []struct {
		name      string
		app, path string
		platform  string
		workers   int
		sdl       []string
		want      []string
	}{
		{
			name: "defaults skip sdl", app: "baka", path: "/games", platform: "Windows",
			want: []string{"install", "baka", "--platform", "Windows", "--base-path", "/games", "--skip-dlcs", "-y", "--skip-sdl"},
		},
		{
			name: "sdl tags mirror heroic emission", app: "baka", path: "/games", platform: "Windows",
			sdl:  []string{"en", "hi"},
			want: []string{"install", "baka", "--platform", "Windows", "--base-path", "/games", "--skip-dlcs", "-y", "--install-tag=", "--install-tag", "en", "--install-tag", "hi"},
		},
		{
			name: "workers", app: "baka", path: "/games", platform: "Mac", workers: 4,
			want: []string{"install", "baka", "--platform", "Mac", "--base-path", "/games", "--skip-dlcs", "-y", "--skip-sdl", "--max-workers", "4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LegendaryInstallArgs(tt.app, tt.path, tt.platform, tt.workers, tt.sdl); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestLegendaryLaunchArgs(t *testing.T) {
	tests := []struct {
		name     string
		app      string
		extra    []string
		wrapper  string
		wine     string
		override string
		language string
		offline  bool
		want     []string
	}{
		{
			name: "plain native launch", app: "baka",
			want: []string{"launch", "baka"},
		},
		{
			name: "wine plus wrapper", app: "baka", wrapper: "mangohud --dlsym", wine: "/usr/bin/wine",
			want: []string{"launch", "baka", "--wine", "/usr/bin/wine", "--wrapper", "mangohud --dlsym"},
		},
		{
			name: "umu proton", app: "baka", wrapper: "umu-run",
			want: []string{"launch", "baka", "--no-wine", "--wrapper", "umu-run"},
		},
		{
			name: "offline language exe", app: "baka", override: "/g/Bin/Alt.exe", language: "de", offline: true,
			want: []string{"launch", "baka", "--override-exe", "/g/Bin/Alt.exe", "--language", "de", "--offline"},
		},
		{
			name: "extra args first", app: "baka", extra: []string{"-windowed"},
			want: []string{"launch", "baka", "-windowed"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LegendaryLaunchArgs(tt.app, tt.extra, tt.wrapper, tt.wine, tt.override, tt.language, tt.offline)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestGogdlDownloadArgs(t *testing.T) {
	got := GogdlDownloadArgs("1207658919", "windows", "/games/Wasabi", "/support/1207658919", "en-US", nil, 0)
	want := []string{"download", "1207658919", "--platform", "windows", "--path", "/games/Wasabi", "--support", "/support/1207658919", "--skip-dlcs", "--lang", "en-US"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	got = GogdlDownloadArgs("1", "windows", "/p", "/s", "", []string{"dlc1", "dlc2"}, 8)
	want = []string{"download", "1", "--platform", "windows", "--path", "/p", "--support", "/s", "--with-dlcs", "--dlcs", "dlc1,dlc2", "--max-workers", "8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestGogdlLaunchArgs(t *testing.T) {
	got := GogdlLaunchArgs("/games/Wasabi", "1207658919", "windows", []string{"-dev"}, "umu-run", "", "")
	want := []string{"launch", "/games/Wasabi", "1207658919", "--no-wine", "--wrapper", "umu-run", "--platform", "windows", "-dev"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	got = GogdlLaunchArgs("/p", "1", "windows", nil, "w", "/usr/bin/wine", "/p/alt.exe")
	want = []string{"launch", "/p", "--override-exe", "/p/alt.exe", "1", "--wine", "/usr/bin/wine", "--wrapper", "w", "--platform", "windows"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestNileLaunchArgs(t *testing.T) {
	got := NileLaunchArgs("amzn-1", "umu-run", "", "/pfx/amzn", "", []string{"-x"})
	want := []string{"launch", "--no-wine", "--wrapper", "umu-run", "--wine-prefix", "/pfx/amzn", "-x", "amzn-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestNileRegisterArgs(t *testing.T) {
	got := NileRegisterArgs("c", "v", "s", "cid")
	want := []string{"register", "--code", "c", "--code-verifier", "v", "--serial", "s", "--client-id", "cid"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}
