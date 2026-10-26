package main

import (
	"encoding/json"
	"flag"
	"reflect"
	"testing"
)

func TestReorderFlags(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	set := fs.String("set", "", "")
	dry := fs.Bool("dry-run", false, "")
	jsonOut := fs.Bool("json", false, "")
	lines := fs.Int("lines", 0, "")
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"epic", "id", "--set", "@/tmp/s.json"}, []string{"--set", "@/tmp/s.json", "epic", "id"}},
		{[]string{"epic", "id", "--dry-run"}, []string{"--dry-run", "epic", "id"}},
		{[]string{"--json", "epic", "id"}, []string{"--json", "epic", "id"}},
		{[]string{"epic", "id", "--set", "-"}, []string{"--set", "-", "epic", "id"}},
		{[]string{"epic", "id", "--set", "@/tmp/s.json", "--json"}, []string{"--set", "@/tmp/s.json", "--json", "epic", "id"}},
		{[]string{"--lines", "3", "--json"}, []string{"--lines", "3", "--json"}},
	}
	for _, c := range cases {
		if got := reorderFlags(fs, c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("reorderFlags(%q): want %q, got %q", c.in, c.want, got)
		}
	}
	if err := fs.Parse(reorderFlags(fs, []string{"--lines", "3", "--json"})); err != nil {
		t.Fatal(err)
	}
	if *lines != 3 || !*jsonOut || *set != "" || *dry {
		t.Fatalf("parsed lines=%d json=%v set=%q dry=%v", *lines, *jsonOut, *set, *dry)
	}
}

func TestHumanUptime(t *testing.T) {
	cases := []struct {
		seconds int64
		want    string
	}{
		{0, "0s"},
		{42, "42s"},
		{312, "5m12s"},
		{7464, "2h4m"},
	}
	for _, c := range cases {
		if got := humanUptime(c.seconds); got != c.want {
			t.Errorf("humanUptime(%d) = %q, want %q", c.seconds, got, c.want)
		}
	}
}

func TestStatusText(t *testing.T) {
	s := statusView{
		Version:       "0.1.0",
		Socket:        "/run/user/1000/hikari.sock",
		UptimeSeconds: 312,
		Engines: map[string]engineView{
			"legendary": {Available: true},
			"wine":      {Available: true},
			"comet":     {Available: false},
		},
		Stores: []statusStoreView{
			{Store: "epic", Available: true, LoggedIn: true, User: "Napim erdem"},
			{Store: "gog"},
			{Store: "steam", Available: true},
		},
	}
	want := "hikari 0.1.0, up 5m12s\n" +
		"socket  /run/user/1000/hikari.sock\n" +
		"engines comet (missing), legendary, wine\n" +
		"stores  epic (Napim erdem), gog (logged out), steam\n"
	if got := statusText(s); got != want {
		t.Fatalf("statusText:\n got %q\nwant %q", got, want)
	}
}

func TestMergeSettingJSON(t *testing.T) {
	base := map[string]any{
		"wine_prefix":  "/pfx/mine",
		"enable_esync": true,
		"eac_runtime":  false,
	}
	merged, err := mergeSettingJSON(base, []byte(`{"eac_runtime":true}`))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(merged, &got); err != nil {
		t.Fatal(err)
	}
	if got["wine_prefix"] != "/pfx/mine" || got["enable_esync"] != true || got["eac_runtime"] != true {
		t.Fatalf("merged settings = %v", got)
	}

	if _, err := mergeSettingJSON(base, []byte(`["not","an","object"]`)); err == nil {
		t.Fatal("a non-object --set should be rejected")
	}
}
