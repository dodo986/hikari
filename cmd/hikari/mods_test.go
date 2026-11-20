package main

import (
	"flag"
	"testing"
)

func installFlags(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	fs := flag.NewFlagSet("mods install", flag.ContinueOnError)
	for _, name := range []string{"profile", "archive", "url", "name", "mod-id", "file-id"} {
		fs.String(name, "", name)
	}
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return installParams(fs, fs.Args())
}

// Nexus ids used to travel as hyphenated strings the daemon never read, so an
// install did nothing at all. Keep the wire shape honest.
func TestInstallParamsSendNexusIDsAsNumbers(t *testing.T) {
	params, err := installFlags(t, "cyberpunk_2077", "--mod-id", "7054", "--file-id", "91915")
	if err != nil {
		t.Fatalf("installParams: %v", err)
	}
	if params["mod_id"] != 7054 || params["file_id"] != 91915 {
		t.Fatalf("ids did not reach the params as numbers: %+v", params)
	}
	if params["game"] != "cyberpunk_2077" {
		t.Fatalf("game = %v", params["game"])
	}
}

func TestInstallParamsTakeOneLinkOrOneArchive(t *testing.T) {
	params, err := installFlags(t, "skyrim_se", "--url", "nxm://skyrimspecialedition/mods/12604/files/1?key=k&expires=2")
	if err != nil {
		t.Fatalf("installParams: %v", err)
	}
	if params["url"] == "" || params["mod_id"] != nil {
		t.Fatalf("url install got %+v", params)
	}

	if _, err := installFlags(t, "skyrim_se", "--mod-id", "12604"); err == nil {
		t.Fatal("a mod id without a file id was accepted")
	}
	if _, err := installFlags(t, "skyrim_se"); err == nil {
		t.Fatal("an install with no source was accepted")
	}
}

// updateFlags mirrors how modsJobCmd hands `mods update` its parsed flags.
func updateFlags(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	fs := flag.NewFlagSet("mods update", flag.ContinueOnError)
	for _, name := range []string{"profile", "name"} {
		fs.String(name, "", name)
	}
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return updateParams(fs, fs.Args())
}

// copyFlags only stores non-empty flags, so an omitted --name left the key
// absent and the guard's `params["name"] == ""` was false: the invalid request
// travelled to the daemon instead of failing locally with its usage error.
func TestUpdateParamsRequireName(t *testing.T) {
	if _, err := updateFlags(t, "cyberpunk_2077"); err == nil {
		t.Fatal("mods update without --name was accepted")
	}

	params, err := updateFlags(t, "cyberpunk_2077", "--name", "Better Sorted Inventory", "--profile", "default")
	if err != nil {
		t.Fatalf("updateParams: %v", err)
	}
	if params["game"] != "cyberpunk_2077" || params["name"] != "Better Sorted Inventory" || params["profile"] != "default" {
		t.Fatalf("update params = %+v", params)
	}
}

func TestNxmDomain(t *testing.T) {
	cases := map[string]string{
		"nxm://cyberpunk2077/mods/7054/files/91915?key=a&expires=1": "cyberpunk2077",
		"nxm://skyrimspecialedition/mods/2014/files/1234":           "skyrimspecialedition",
		"https://www.nexusmods.com/cyberpunk2077/mods/7054":         "",
		"nxm://":      "",
		"nxm://nomod": "",
	}
	for link, want := range cases {
		if got := nxmDomain(link); got != want {
			t.Errorf("nxmDomain(%q) = %q, want %q", link, got, want)
		}
	}
}
