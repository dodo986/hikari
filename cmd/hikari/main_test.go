package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"hikari/internal/api"
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

func TestReorderFlagsDashDashAndUnknownFlags(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.Bool("json", false, "")
	fs.Bool("dry-run", false, "")
	cases := []struct {
		in   []string
		want []string
	}{
		// "--" ends Hikari's flags: it and everything after it stay positional.
		{[]string{"epic", "id", "--", "-foo", "--json"}, []string{"epic", "id", "--", "-foo", "--json"}},
		{[]string{"--json", "epic", "id", "--", "--dry-run"}, []string{"--json", "epic", "id", "--", "--dry-run"}},
		// An unknown dash token stays a flag, so flag.Parse reports a typo
		// instead of the command ignoring it.
		{[]string{"epic", "42", "--keep-file"}, []string{"--keep-file", "epic", "42"}},
		{[]string{"epic", "id", "-foo", "--json"}, []string{"-foo", "--json", "epic", "id"}},
	}
	for _, c := range cases {
		if got := reorderFlags(fs, c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("reorderFlags(%q): want %q, got %q", c.in, c.want, got)
		}
	}

	// `hikari launch` is the exception: its dash tokens belong to the game.
	gameCases := []struct {
		in   []string
		want []string
	}{
		{[]string{"epic", "id", "-screen-fullscreen"}, []string{"epic", "id", "-screen-fullscreen"}},
		{[]string{"epic", "id", "-foo", "--json"}, []string{"--json", "epic", "id", "-foo"}},
		{[]string{"epic", "id", "--", "--json", "-foo"}, []string{"epic", "id", "--", "--json", "-foo"}},
	}
	for _, c := range gameCases {
		if got := reorderGameArgs(fs, c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("reorderGameArgs(%q): want %q, got %q", c.in, c.want, got)
		}
	}
}

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"simple", "simple"},
		{"/usr/bin/game", "/usr/bin/game"},
		{"a-b_c@d%e+f=g:h,i.j/k", "a-b_c@d%e+f=g:h,i.j/k"},
		{"", "''"},
		{"two words", "'two words'"},
		{"it's", `'it'\''s'`},
		{"a;b", "'a;b'"},
		{"$HOME", "'$HOME'"},
		{"`id`", "'`id`'"},
		{"a>b", "'a>b'"},
		{"a&b", "'a&b'"},
		{"a|b", "'a|b'"},
		{"(x)", "'(x)'"},
		{"*", "'*'"},
		{"?", "'?'"},
		{"#x", "'#x'"},
		{"a\nb", "'a\nb'"},
		{"it's a $test; rm -rf /", `'it'\''s a $test; rm -rf /'`},
	}
	for _, c := range cases {
		if got := shellQuote(c.in); got != c.want {
			t.Errorf("shellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

type stubCall struct {
	method string
	params map[string]any
}

// stubDaemon answers JSON-RPC requests over a unix socket with an empty result
// and, for methods registered with replyAfter, one extra event line per entry
// once the reply is on the wire.
type stubDaemon struct {
	path  string
	mu    sync.Mutex
	calls []stubCall
	after map[string][]string
}

func startStubDaemon(t *testing.T) *stubDaemon {
	t.Helper()
	d := &stubDaemon{path: filepath.Join(t.TempDir(), "stub.sock"), after: map[string][]string{}}
	ln, err := net.Listen("unix", d.path)
	if err != nil {
		t.Fatal(err)
	}
	go d.serve(ln)
	t.Cleanup(func() { _ = ln.Close() })
	return d
}

func (d *stubDaemon) replyAfter(method, event string) {
	d.mu.Lock()
	d.after[method] = append(d.after[method], event)
	d.mu.Unlock()
}

func (d *stubDaemon) serve(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go d.handle(conn)
	}
}

func (d *stubDaemon) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		var req api.Request
		if json.Unmarshal([]byte(line), &req) != nil {
			continue
		}
		var params map[string]any
		_ = json.Unmarshal(req.Params, &params)
		d.mu.Lock()
		d.calls = append(d.calls, stubCall{method: req.Method, params: params})
		events := append([]string(nil), d.after[req.Method]...)
		d.mu.Unlock()

		resp, err := json.Marshal(&api.Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}})
		if err != nil {
			return
		}
		if _, err := conn.Write(append(resp, '\n')); err != nil {
			return
		}
		for _, ev := range events {
			if _, err := conn.Write([]byte(ev + "\n")); err != nil {
				return
			}
		}
	}
}

func (d *stubDaemon) find(method string) (stubCall, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range d.calls {
		if c.method == method {
			return c, true
		}
	}
	return stubCall{}, false
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	callErr := fn()
	_ = w.Close()
	out, _ := io.ReadAll(r)
	_ = r.Close()
	return string(out), callErr
}

// hikari uninstall <store> <id> --keep-files must not delete the game files:
// the flag used to sit after the positionals and was dropped by flag.Parse.
func TestUninstallKeepFilesAfterPositionals(t *testing.T) {
	d := startStubDaemon(t)
	if err := dispatch("uninstall", []string{"--socket", d.path, "epic", "id", "--keep-files"}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	call, ok := d.find("game.uninstall")
	if !ok {
		t.Fatal("game.uninstall was never called")
	}
	if call.params["keep_files"] != true {
		t.Fatalf("keep_files = %v, want true", call.params["keep_files"])
	}
}

// The other commands that ignored flags after positionals get the same
// treatment; --json is the flag every one of them shares.
func TestFlagsAfterPositionalsAreParsed(t *testing.T) {
	d := startStubDaemon(t)
	out, err := captureStdout(t, func() error {
		return dispatch("options", []string{"--socket", d.path, "epic", "id", "--json"})
	})
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	if !strings.Contains(out, "{") {
		t.Fatalf("options --json printed %q, want JSON", out)
	}
}

func TestFollowTerminalStateIsPropagated(t *testing.T) {
	cases := []struct {
		name   string
		cmd    string
		args   []string
		method string
		event  string
	}{
		{"install error", "install", []string{"--path", "/tmp/game", "epic", "id", "--follow"}, "game.install",
			`{"jsonrpc":"2.0","method":"install.progress","params":{"state":"error"}}`},
		{"update canceled", "update", []string{"epic", "id", "--follow"}, "game.update",
			`{"jsonrpc":"2.0","method":"install.progress","params":{"state":"canceled"}}`},
		{"repair error", "repair", []string{"epic", "id", "--follow"}, "game.repair",
			`{"jsonrpc":"2.0","method":"install.progress","params":{"state":"error"}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := startStubDaemon(t)
			d.replyAfter(c.method, c.event)
			args := append([]string{"--socket", d.path}, c.args...)
			if err := dispatch(c.cmd, args); err == nil {
				t.Fatalf("%s --follow returned success after the job failed", c.cmd)
			}
		})
	}
}

func TestFollowDoneIsSuccess(t *testing.T) {
	d := startStubDaemon(t)
	d.replyAfter("game.install", `{"jsonrpc":"2.0","method":"install.progress","params":{"state":"done"}}`)
	if err := dispatch("install", []string{"--socket", d.path, "--path", "/tmp/game", "epic", "id", "--follow"}); err != nil {
		t.Fatalf("install --follow done = %v, want nil", err)
	}
}

func TestWineInstallFollowFailsOnError(t *testing.T) {
	d := startStubDaemon(t)
	d.replyAfter("wine.download", `{"jsonrpc":"2.0","method":"wine.progress","params":{"phase":"error","done":true,"error":"no such release"}}`)
	err := dispatch("wine", []string{"install", "--socket", d.path, "GE-Proton9", "--follow"})
	if err == nil {
		t.Fatal("wine install --follow returned success after the download failed")
	}
	if !strings.Contains(err.Error(), "no such release") {
		t.Fatalf("wine install --follow error = %v", err)
	}
}

func TestWineInstallFollowDoneIsSuccess(t *testing.T) {
	d := startStubDaemon(t)
	d.replyAfter("wine.download", `{"jsonrpc":"2.0","method":"wine.progress","params":{"phase":"done","done":true,"percent":100}}`)
	if err := dispatch("wine", []string{"install", "--socket", d.path, "GE-Proton9", "--follow"}); err != nil {
		t.Fatalf("wine install --follow done = %v, want nil", err)
	}
}

func TestLaunchDashArgsAreGameArgs(t *testing.T) {
	d := startStubDaemon(t)
	if err := dispatch("launch", []string{"--socket", d.path, "epic", "id", "-screen-fullscreen"}); err != nil {
		t.Fatalf("launch: %v", err)
	}
	call, ok := d.find("game.launch")
	if !ok {
		t.Fatal("game.launch was never called")
	}
	if got := call.params["args"]; !reflect.DeepEqual(got, []any{"-screen-fullscreen"}) {
		t.Fatalf("launch args = %v, want [-screen-fullscreen]", got)
	}
}

func TestLaunchSeparatorKeepsDashFlagsAsArgs(t *testing.T) {
	d := startStubDaemon(t)
	if err := dispatch("launch", []string{"--socket", d.path, "epic", "id", "--", "--json"}); err != nil {
		t.Fatalf("launch: %v", err)
	}
	call, ok := d.find("game.launch")
	if !ok {
		t.Fatal("game.launch was never called")
	}
	if got := call.params["args"]; !reflect.DeepEqual(got, []any{"--json"}) {
		t.Fatalf("launch args = %v, want [--json]", got)
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
