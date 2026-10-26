package opti

import (
	"context"
	"testing"
)

var _ Manager = (*Fake)(nil)

func TestFakeLifecycle(t *testing.T) {
	f := NewFake()
	ctx := context.Background()

	gpus, err := f.DetectGPUs(ctx)
	if err != nil || len(gpus) != 1 || !gpus[0].Primary {
		t.Fatalf("gpu detect: %+v %v", gpus, err)
	}

	releases, err := f.ListReleases(ctx)
	if err != nil || len(releases) != 1 || releases[0].Tag != "v0.6.9" {
		t.Fatalf("releases: %+v %v", releases, err)
	}

	var frames []Progress
	if err := f.Download(ctx, "v0.6.9", func(p Progress) { frames = append(frames, p) }); err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 || !frames[1].Done {
		t.Fatalf("download frames: %+v", frames)
	}
	versions, _ := f.ListVersions(ctx)
	if len(versions) != 1 || versions[0] != "v0.6.9" {
		t.Fatalf("versions after download: %+v", versions)
	}

	var installFrames []Progress
	err = f.Install(ctx, InstallOptions{
		GameID: "1423049311", ExePath: "/games/CP2077/bin/x64/Cyberpunk2077.exe",
		InstallDir: "/games/CP2077", Version: "v0.6.9",
		Upscaler: "fsr", FrameGen: true, Int8: true,
	}, func(p Progress) { installFrames = append(installFrames, p) })
	if err != nil {
		t.Fatal(err)
	}
	if len(installFrames) != 2 || !installFrames[len(installFrames)-1].Done {
		t.Fatalf("install frames: %+v", installFrames)
	}
	installs := f.Installs()
	if len(installs) != 1 || installs[0].Injection != "dxgi.dll" {
		t.Fatalf("injection default not applied: %+v", installs)
	}

	st, err := f.Status(ctx, "1423049311", "", "")
	if err != nil || len(st) != 1 || !st[0].Installed || st[0].GameDir != "/games/CP2077/bin/x64" {
		t.Fatalf("status after install: %+v %v", st, err)
	}

	all, _ := f.Status(ctx, "", "", "")
	if len(all) != 1 {
		t.Fatalf("status-all: %+v", all)
	}

	if err := f.Uninstall(ctx, "1423049311", "/games/CP2077/bin/x64/Cyberpunk2077.exe", "/games/CP2077"); err != nil {
		t.Fatal(err)
	}
	if st, _ := f.Status(ctx, "1423049311", "", ""); len(st) != 0 {
		t.Fatalf("status after uninstall: %+v", st)
	}
	if err := f.Uninstall(ctx, "1423049311", "", ""); err == nil {
		t.Fatal("uninstalling an uninstalled game should fail")
	}
}

func TestFakeValidationAndFailure(t *testing.T) {
	f := NewFake()
	ctx := context.Background()
	if err := f.Install(ctx, InstallOptions{GameID: "g"}, nil); err == nil {
		t.Fatal("install without exe/version should fail")
	}
	f.FailOn = "install"
	if err := f.Install(ctx, InstallOptions{GameID: "g", ExePath: "/e", Version: "v"}, nil); err == nil {
		t.Fatal("FailOn=install must fail")
	}
}
