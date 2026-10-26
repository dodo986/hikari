package main

import (
	"testing"
	"time"

	"hikari/internal/opti"
)

func TestOptiIPCEndToEndDemo(t *testing.T) {
	d, client := startDemoDaemon(t)
	_ = d

	var detect struct {
		GPUs []opti.GPU `json:"gpus"`
	}
	if err := client.Call("opti.detect", nil, &detect); err != nil {
		t.Fatal(err)
	}
	if len(detect.GPUs) != 1 || !detect.GPUs[0].Primary {
		t.Fatalf("detect: %+v", detect.GPUs)
	}

	var st opti.Status
	if err := client.Call("opti.status", map[string]any{"store": "epic", "id": "demo-epic-1"}, &st); err != nil {
		t.Fatal(err)
	}
	if st.Installed {
		t.Fatalf("should start uninstalled: %+v", st)
	}

	var rel struct {
		Releases []opti.Release `json:"releases"`
	}
	if err := client.Call("opti.releases", nil, &rel); err != nil {
		t.Fatal(err)
	}
	if len(rel.Releases) == 0 || rel.Releases[0].Tag != "v0.6.9" {
		t.Fatalf("releases: %+v", rel.Releases)
	}

	var res map[string]any
	if err := client.Call("opti.install", map[string]any{
		"store": "epic", "id": "demo-epic-1", "tag": "v0.6.9",
	}, &res); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := client.Call("opti.status", map[string]any{"store": "epic", "id": "demo-epic-1"}, &st); err != nil {
			t.Fatal(err)
		}
		if st.Installed {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !st.Installed || st.Version != "v0.6.9" || st.InjectionDLL != "dxgi.dll" {
		t.Fatalf("install did not land: %+v", st)
	}

	if err := client.Call("opti.uninstall", map[string]any{"store": "epic", "id": "demo-epic-1"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := client.Call("opti.status", map[string]any{"store": "epic", "id": "demo-epic-1"}, &st); err != nil {
		t.Fatal(err)
	}
	if st.Installed {
		t.Fatalf("uninstall did not clear: %+v", st)
	}

	err := client.Call("opti.uninstall", map[string]any{"store": "epic", "id": "demo-epic-1"}, nil)
	if err == nil {
		t.Fatal("uninstall of an uninstalled game must fail")
	}
}
