package opti

import "testing"

func TestHookName(t *testing.T) {
	cases := map[string]string{
		"":               "dxgi.dll",
		"dxgi":           "dxgi.dll",
		"dxgi.dll":       "dxgi.dll",
		"winmm":          "winmm.dll",
		"OptiScaler.asi": "OptiScaler.asi",
		" DXVK ":         "DXVK.dll",
	}
	for in, want := range cases {
		if got := HookName(in); got != want {
			t.Errorf("HookName(%q): want %q, got %q", in, want, got)
		}
	}
	if !ValidInjection(HookName("wininet")) {
		t.Fatal("normalised short name must be a valid hook")
	}
}
