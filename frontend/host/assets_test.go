package main

import "testing"

func TestLinkIn(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"nothing", []string{"--flag"}, ""},
		{"bare link", []string{"nxm://skyrimspecialedition/mods/1/files/2?key=k"}, "nxm://skyrimspecialedition/mods/1/files/2?key=k"},
		{"after a flag", []string{"--verbose", "nxm://cyberpunk2077/mods/3/files/4"}, "nxm://cyberpunk2077/mods/3/files/4"},
		{"http is not a link", []string{"https://nexusmods.com/mods/1"}, ""},
	}
	for _, tc := range cases {
		if got := linkIn(tc.args); got != tc.want {
			t.Errorf("%s: linkIn(%v) = %q, want %q", tc.name, tc.args, got, tc.want)
		}
	}
}
