package engine

import (
	"math"
	"testing"
)

func TestParserLegendarySequence(t *testing.T) {
	p := &Parser{}
	p.SetTotalMiB(2048)

	lines := []string{
		"[cli] INFO: Download size: 2048.00 MiB",
		"[cli] INFO: Starting download...",
		"Downloaded: 10.24 MiB",
		"Download\t- 5.12 MiB/s",
		"Disk\t- 4.00 MiB/s",
		"ETA: 00:06:30",
		"Downloaded: 512.00 MiB",
	}
	var frames []Frame
	for _, l := range lines {
		if f, emit := p.Parse(l); emit {
			frames = append(frames, f)
		}
	}
	if len(frames) != 2 {
		t.Fatalf("want 2 frames, got %d: %+v", len(frames), frames)
	}
	first := frames[0]
	if first.Bytes != "10.24MB" {
		t.Errorf("first frame bytes = %q, want 10.24MB", first.Bytes)
	}
	if first.Percent == nil || !almost(*first.Percent, 0.5) {
		t.Errorf("first frame percent = %v, want 0.5", first.Percent)
	}
	second := frames[1]
	if second.Bytes != "512.00MB" {
		t.Errorf("second frame bytes = %q, want 512.00MB", second.Bytes)
	}
	if second.ETA != "00:06:30" {
		t.Errorf("second frame eta = %q", second.ETA)
	}
	if second.DownSpeed == nil || !almost(*second.DownSpeed, 5.12) {
		t.Errorf("second frame down speed = %v, want 5.12", second.DownSpeed)
	}
	if second.DiskSpeed == nil || !almost(*second.DiskSpeed, 4.0) {
		t.Errorf("second frame disk speed = %v, want 4.0", second.DiskSpeed)
	}
	if second.Percent == nil || !almost(*second.Percent, 25.0) {
		t.Errorf("second frame percent = %v, want 25", second.Percent)
	}
	if second.DownloadSizeMiB == nil || !almost(*second.DownloadSizeMiB, 2048) {
		t.Errorf("download size not carried: %+v", second.DownloadSizeMiB)
	}
}

func TestParserGogdlPercentLine(t *testing.T) {
	p := &Parser{}
	lines := []string{
		"Progress: 42.5 ",
		"Downloaded: 870.40 MiB",
		"ETA: 00:10:00",
		"Download\t- 2.00 MiB",
		"Disk\t- 1.50 MiB",
		"Progress: 43.1 ",
		"Downloaded: 882.00 MiB",
	}
	var frames []Frame
	for _, l := range lines {
		if f, emit := p.Parse(l); emit {
			frames = append(frames, f)
		}
	}
	if len(frames) != 2 {
		t.Fatalf("want 2 frames, got %d", len(frames))
	}
	if frames[1].Percent == nil || !almost(*frames[1].Percent, 43.1) {
		t.Errorf("printed percent should win: %+v", frames[1].Percent)
	}
	if frames[0].Percent == nil || !almost(*frames[0].Percent, 42.5) {
		t.Errorf("percent = %+v, want 42.5", frames[0].Percent)
	}
}

func TestParserIgnoresNoise(t *testing.T) {
	p := &Parser{}
	for _, l := range []string{
		"[DLManager] INFO: Using secure connection...",
		"[cli] INFO: This is a test message with ETA-like text: 12:00:00",
		"Resuming download...",
	} {
		if _, emit := p.Parse(l); emit {
			t.Errorf("noise line emitted a frame: %q", l)
		}
	}
}

func TestParserCapsPercentAt100(t *testing.T) {
	p := &Parser{}
	p.SetTotalMiB(100)
	if f, emit := p.Parse("Downloaded: 150.00 MiB"); !emit {
		t.Fatal("expected emit")
	} else if f.Percent == nil || *f.Percent != 100 {
		t.Fatalf("percent = %v, want capped 100", f.Percent)
	}
}

func almost(a, b float64) bool { return math.Abs(a-b) < 0.001 }
