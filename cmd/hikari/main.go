package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"hikari/internal/api"
	"hikari/internal/config"
	"hikari/internal/logging"
)

const usage = `hikari - control the Hikari game launcher daemon

Usage:
  hikari status                          daemon status
  hikari list [--store S] [--installed]  list the library
  hikari get <store> <id>                one game
  hikari refresh [store]                 re-sync library from the stores
  hikari launch <store> <id> [args...]   launch (--dry-run prints the plan)
  hikari logs [--lines N] [--game STORE ID]
                                         daemon log, or one game's last launch
  hikari report                          diagnostics bundle to paste in a report
  hikari update-check                    is a newer GitHub release available?
  hikari update-apply                    download and install it in place
  hikari stop <store> <id>               stop a running game
  hikari install <store> <id> --path P [--platform P] [--workers N] [--lang L] [--follow]
  hikari update <store> <id> [--workers N] [--follow]
  hikari options <store> <id>            the game's launch options
  hikari favorite <store> <id> [on|off]  pin/unpin a game
  hikari add-local <title> <exe> [--platform Linux|Windows] [--artwork PATH]
  hikari remove-local <id>               forget a local game
  hikari repair <store> <id>             verify and repair the install
  hikari saves <store> <id> download|upload
  hikari uninstall <store> <id> [--keep-files]
  hikari settings <store> <id> [--set <json|@file|->]
  hikari auth status
  hikari auth login <store> [code]       without code: prints the login URL
  hikari auth logout <store>
  hikari engines                         engine availability and versions
  hikari wines                           installed Wine/Proton builds
  hikari wine releases                   downloadable Proton builds
  hikari wine install <tag> [--follow]   download + verify + extract a build
  hikari wine remove <name>              delete a Hikari-installed build
  hikari mods status                     is the Amethyst modding backend ready?
  hikari mods setup                      install Amethyst's Python packages for Hikari
  hikari mods games                      games Amethyst can mod, with their profiles
  hikari mods deploy <game> [--profile P]  apply a profile to the game
  hikari mods restore <game>             strip the deployed mods again
  hikari mods install <game> --archive PATH | --url nxm://…  [--profile P] [--name N]
                                         install one mod archive (FOMOD: defaults)
  hikari mods cancel                     stop the running deploy/install
  hikari mods login                      sign in to Nexus in your browser
  hikari mods logout                     forget the browser login
  hikari mods account                    who is signed in, and are they premium?
  hikari mods browse <game> [query] [--sort downloads|endorsements|created|updated]
                                         search a game's mods on Nexus
  hikari mods files <game> <mod-id>      the files a browsed mod offers
  hikari mods installed <game> [--profile P]  the profile's modlist, in order
  hikari mods nexus [status|set|clear]   a Nexus API key, if you have one instead
  hikari shutdown                        stop the daemon

Every command also accepts --json and --socket PATH.
Stores: epic, gog, amazon, steam. Games discovered in the Heroic launcher
appear under their real store (epic/gog/amazon); they launch, but install
and uninstall belong to the launcher that owns them.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		fmt.Print(usage)
		return
	}
	if err := dispatch(cmd, args); err != nil {
		if re, ok := err.(*api.Error); ok {
			fmt.Fprintf(os.Stderr, "hikari: %s\n", re.Message)
		} else {
			fmt.Fprintf(os.Stderr, "hikari: %v\n", err)
		}
		os.Exit(1)
	}
}

type commonFlags struct {
	json   bool
	socket string
}

func addCommon(fs *flag.FlagSet) *commonFlags {
	c := &commonFlags{}
	fs.BoolVar(&c.json, "json", false, "print the raw JSON reply")
	fs.StringVar(&c.socket, "socket", "", "daemon socket path")
	return c
}

func dial(c *commonFlags) *api.Client {
	client, err := dialQuiet(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hikari:", err)
		os.Exit(1)
	}
	return client
}

func dialQuiet(c *commonFlags) (*api.Client, error) {
	path := c.socket
	if path == "" {
		path = config.SocketPath()
	}
	return api.Dial(path)
}

func (c *commonFlags) printRaw(raw json.RawMessage) error {
	var buf bytes.Buffer
	if json.Indent(&buf, raw, "", "  ") == nil {
		fmt.Println(buf.String())
		return nil
	}
	fmt.Println(string(raw))
	return nil
}

func (c *commonFlags) print(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func dispatch(cmd string, args []string) error {
	switch cmd {
	case "status":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		if c.json {
			var raw json.RawMessage
			if err := dial(c).Call("daemon.status", nil, &raw); err != nil {
				return err
			}
			return c.printRaw(raw)
		}
		var res statusView
		if err := dial(c).Call("daemon.status", nil, &res); err != nil {
			return err
		}
		fmt.Print(statusText(res))
		return nil

	case "list":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		store := fs.String("store", "", "filter by store")
		installed := fs.Bool("installed", false, "installed games only")
		if err := fs.Parse(args); err != nil {
			return err
		}
		params := map[string]any{}
		if *store != "" {
			params["store"] = *store
		}
		if *installed {
			params["installed_only"] = true
		}
		if c.json {
			var raw json.RawMessage
			if err := dial(c).Call("library.list", params, &raw); err != nil {
				return err
			}
			return c.printRaw(raw)
		}
		var res struct {
			Games []gameView `json:"games"`
		}
		if err := dial(c).Call("library.list", params, &res); err != nil {
			return err
		}
		for _, g := range res.Games {
			mark := " "
			if g.Installed {
				mark = "*"
			}
			fmt.Printf("%s %-7s %-24s %s\n", mark, g.Store, g.ID, g.Title)
		}
		return nil

	case "get":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 2 {
			return fmt.Errorf("usage: hikari get <store> <id>")
		}
		var res json.RawMessage
		if err := dial(c).Call("library.get", map[string]any{"store": pos[0], "id": pos[1]}, &res); err != nil {
			return err
		}
		return c.printRaw(res)

	case "refresh":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		params := map[string]any{}
		if len(fs.Args()) > 0 {
			params["store"] = fs.Args()[0]
		}
		var res map[string]any
		if err := dial(c).Call("library.refresh", params, &res); err != nil {
			return err
		}
		return c.print(res)

	case "launch":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		dryRun := fs.Bool("dry-run", false, "print the exact command instead of running it")
		if err := fs.Parse(reorderFlags(fs, args)); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 2 {
			return fmt.Errorf("usage: hikari launch <store> <id> [game args...]")
		}
		params := map[string]any{"store": pos[0], "id": pos[1], "dry_run": *dryRun}
		if len(pos) > 2 {
			params["args"] = pos[2:]
		}
		var res struct {
			Plan  *planView `json:"plan,omitempty"`
			State string    `json:"state,omitempty"`
		}
		if err := dial(c).Call("game.launch", params, &res); err != nil {
			return err
		}
		if c.json {
			return c.print(res)
		}
		if res.Plan != nil {
			printPlan(res.Plan)
			return nil
		}
		fmt.Printf("%s/%s: %s\n", pos[0], pos[1], res.State)
		return watchLaunch(dial(c), pos[0], pos[1])

	case "logs":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		lines := fs.Int("lines", 120, "how many lines")
		game := fs.Bool("game", false, "the game's last launch instead of the daemon log")
		if err := fs.Parse(reorderFlags(fs, args)); err != nil {
			return err
		}
		if *game {
			pos := fs.Args()
			if len(pos) < 2 {
				return fmt.Errorf("usage: hikari logs --game <store> <id>")
			}
			var rec struct {
				Lines    []string `json:"lines"`
				Error    string   `json:"error"`
				ExitCode int      `json:"exit_code"`
				Running  bool     `json:"running"`
				State    string   `json:"state"`
			}
			if err := dial(c).Call("game.log", map[string]any{"store": pos[0], "id": pos[1]}, &rec); err != nil {
				return err
			}
			if c.json {
				return c.print(rec)
			}
			if rec.Error != "" {
				fmt.Fprintf(os.Stderr, "%s/%s crashed: %s (exit %d)\n", pos[0], pos[1], rec.Error, rec.ExitCode)
			}
			for _, line := range rec.Lines {
				fmt.Println(line)
			}
			return nil
		}
		var res struct {
			Path  string   `json:"path"`
			Lines []string `json:"lines"`
		}
		path := config.Resolve().Log
		if client, err := dialQuiet(c); err == nil {
			var reply struct {
				Path  string   `json:"path"`
				Lines []string `json:"lines"`
			}
			if err := client.Call("logs.tail", map[string]any{"lines": *lines}, &reply); err == nil {
				res.Path, res.Lines = reply.Path, reply.Lines
			}
		}
		if res.Path == "" {
			fileLines, ferr := logging.TailFile(path, *lines)
			if ferr != nil {
				return fmt.Errorf("the daemon is not running and %s cannot be read: %w", path, ferr)
			}
			res.Path, res.Lines = path, fileLines
		}
		if c.json {
			return c.print(res)
		}
		fmt.Fprintf(os.Stderr, "# %s\n", res.Path)
		for _, line := range res.Lines {
			fmt.Println(line)
		}
		return nil

	case "update-check":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		var info struct {
			Current   string `json:"current"`
			Latest    string `json:"latest"`
			Available bool   `json:"available"`
			URL       string `json:"url"`
			Notes     string `json:"notes"`
		}
		if err := dial(c).Call("app.update.check", nil, &info); err != nil {
			return err
		}
		if c.json {
			return c.print(info)
		}
		if info.Available {
			fmt.Printf("update available: %s -> %s\n%s\n", info.Current, info.Latest, info.URL)
		} else {
			fmt.Printf("up to date (%s)\n", info.Current)
		}
		return nil

	case "update-apply":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		client := dial(c)
		if err := client.Subscribe("update.progress"); err != nil {
			return err
		}
		var res map[string]any
		if err := client.Call("app.update.apply", nil, &res); err != nil {
			return err
		}
		if c.json {
			return c.print(res)
		}
		fmt.Println("updating…")
		for {
			params, err := client.AwaitEvent("update.progress", 10*time.Minute)
			if err != nil {
				fmt.Println("the daemon restarted to finish the update")
				return nil
			}
			var p struct {
				Phase   string   `json:"phase"`
				Percent *float64 `json:"percent"`
				Error   string   `json:"error"`
			}
			if json.Unmarshal(params, &p) != nil {
				continue
			}
			switch p.Phase {
			case "downloading":
				if p.Percent != nil {
					fmt.Printf("\r  downloading %.1f%%   ", *p.Percent)
				}
			case "installing":
				fmt.Println("\n  installing")
			case "error":
				return fmt.Errorf("%s", p.Error)
			default:
				fmt.Printf("  %s\n", p.Phase)
			}
		}

	case "report":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		var res struct {
			Text string `json:"text"`
		}
		if err := dial(c).Call("logs.report", nil, &res); err != nil {
			return err
		}
		if c.json {
			return c.print(res)
		}
		fmt.Print(res.Text)
		return nil

	case "stop":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 2 {
			return fmt.Errorf("usage: hikari stop <store> <id>")
		}
		var res map[string]any
		if err := dial(c).Call("game.stop", map[string]any{"store": pos[0], "id": pos[1]}, &res); err != nil {
			return err
		}
		return c.print(res)

	case "install":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		path := fs.String("path", "", "install path (required)")
		platform := fs.String("platform", "", "Windows, Mac or Linux")
		workers := fs.Int("workers", 0, "download workers")
		lang := fs.String("lang", "", "install language")
		follow := fs.Bool("follow", false, "stream install progress")
		if err := fs.Parse(reorderFlags(fs, args)); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 2 || *path == "" {
			return fmt.Errorf("usage: hikari install <store> <id> --path PATH [flags]")
		}
		params := map[string]any{"store": pos[0], "id": pos[1], "path": *path}
		if *platform != "" {
			params["platform"] = *platform
		}
		if *workers > 0 {
			params["max_workers"] = *workers
		}
		if *lang != "" {
			params["language"] = *lang
		}
		client := dial(c)
		if *follow {
			client.OnEvent = progressPrinter()
			if err := client.Subscribe("install.progress"); err != nil {
				return err
			}
		}
		var res map[string]any
		if err := client.Call("game.install", params, &res); err != nil {
			return err
		}
		fmt.Println("install started:", res["job"])
		if *follow {
			waitProgressDone(client)
		}
		return nil

	case "update":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		workers := fs.Int("workers", 0, "download workers")
		follow := fs.Bool("follow", false, "stream update progress")
		if err := fs.Parse(reorderFlags(fs, args)); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 2 {
			return fmt.Errorf("usage: hikari update <store> <id>")
		}
		params := map[string]any{"store": pos[0], "id": pos[1]}
		if *workers > 0 {
			params["max_workers"] = *workers
		}
		client := dial(c)
		if *follow {
			client.OnEvent = progressPrinter()
			if err := client.Subscribe("install.progress"); err != nil {
				return err
			}
		}
		var res map[string]any
		if err := client.Call("game.update", params, &res); err != nil {
			return err
		}
		fmt.Println("update started:", res["job"])
		if *follow {
			waitProgressDone(client)
		}
		return nil

	case "uninstall":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		keep := fs.Bool("keep-files", false, "remove the record but keep game files")
		if err := fs.Parse(args); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 2 {
			return fmt.Errorf("usage: hikari uninstall <store> <id>")
		}
		var res map[string]any
		if err := dial(c).Call("game.uninstall", map[string]any{
			"store": pos[0], "id": pos[1], "keep_files": *keep,
		}, &res); err != nil {
			return err
		}
		return c.print(res)

	case "settings":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		set := fs.String("set", "", "new settings as JSON, @file or - for stdin")
		if err := fs.Parse(reorderFlags(fs, args)); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 2 {
			return fmt.Errorf("usage: hikari settings <store> <id> [--set <json|@file|->]")
		}
		store, id := pos[0], pos[1]
		if *set == "" {
			var res map[string]any
			if err := dial(c).Call("game.settings.get", map[string]any{"store": store, "id": id}, &res); err != nil {
				return err
			}
			return c.print(res)
		}
		raw, err := readSettingValue(*set)
		if err != nil {
			return err
		}

		var base map[string]any
		if err := dial(c).Call("game.settings.get", map[string]any{
			"store": store, "id": id,
		}, &base); err != nil {
			return err
		}
		merged, err := mergeSettingJSON(base, raw)
		if err != nil {
			return err
		}
		var res map[string]any
		if err := dial(c).Call("game.settings.set", map[string]any{
			"store": store, "id": id, "settings": json.RawMessage(merged),
		}, &res); err != nil {
			return err
		}
		return c.print(res)

	case "auth":
		return authCmd(args)

	case "add-local":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		platform := fs.String("platform", "Windows", "Linux or Windows")
		artwork := fs.String("artwork", "", "cover image path")
		argsFlag := fs.String("args", "", "game arguments")
		if err := fs.Parse(reorderFlags(fs, args)); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 2 {
			return fmt.Errorf("usage: hikari add-local <title> <executable> [--platform Linux|Windows] [--artwork PATH] [--args \"...\"]")
		}
		params := map[string]any{
			"title": pos[0], "executable": pos[1], "platform": *platform,
		}
		if *artwork != "" {
			params["artwork"] = *artwork
		}
		if *argsFlag != "" {
			params["args"] = strings.Fields(*argsFlag)
		}
		var g struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Platform string `json:"platform"`
		}
		if err := dial(c).Call("game.add_local", params, &g); err != nil {
			return err
		}
		if c.json {
			return c.print(g)
		}
		if g.Platform != "" && g.Platform != *platform {
			fmt.Printf("added %s (%s) · detected %s from the executable\n", g.Title, g.ID, g.Platform)
			return nil
		}
		fmt.Println("added", g.Title, "("+g.ID+")")
		return nil

	case "remove-local":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 1 {
			return fmt.Errorf("usage: hikari remove-local <id>")
		}
		if err := dial(c).Call("game.remove_local", map[string]any{"id": pos[0]}, nil); err != nil {
			return err
		}
		fmt.Println("removed", pos[0])
		return nil

	case "favorite":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 2 || len(pos) > 3 {
			return fmt.Errorf("usage: hikari favorite <store> <id> [on|off]")
		}
		params := map[string]any{"store": pos[0], "id": pos[1]}
		if len(pos) == 3 {
			params["favorite"] = pos[2] == "on" || pos[2] == "true"
		}
		var res map[string]any
		if err := dial(c).Call("game.favorite", params, &res); err != nil {
			return err
		}
		if res["favorite"] == true {
			fmt.Println("pinned")
		} else {
			fmt.Println("unpinned")
		}
		return nil

	case "options":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 2 {
			return fmt.Errorf("usage: hikari options <store> <id>")
		}
		var res struct {
			Options []struct {
				Name   string   `json:"name"`
				Type   string   `json:"type"`
				Params []string `json:"params"`
				Exe    string   `json:"exe"`
				DLCApp string   `json:"dlc_app"`
			} `json:"options"`
		}
		if err := dial(c).Call("game.launch_options", map[string]any{"store": pos[0], "id": pos[1]}, &res); err != nil {
			return err
		}
		if c.json {
			return c.print(res)
		}
		if len(res.Options) == 0 {
			fmt.Println("no extra launch options for this game")
			return nil
		}
		for i, o := range res.Options {
			fmt.Printf("[%d] %-32s %s %s\n", i, o.Name, o.Type, strings.Join(o.Params, " "))
		}
		return nil

	case "repair":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		follow := fs.Bool("follow", false, "stream repair progress")
		if err := fs.Parse(reorderFlags(fs, args)); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 2 {
			return fmt.Errorf("usage: hikari repair <store> <id> [--follow]")
		}
		client := dial(c)
		if *follow {
			client.OnEvent = progressPrinter()
			if err := client.Subscribe("install.progress"); err != nil {
				return err
			}
		}
		var res map[string]any
		if err := client.Call("game.repair", map[string]any{"store": pos[0], "id": pos[1]}, &res); err != nil {
			return err
		}
		fmt.Println("repair started:", res["job"])
		if *follow {
			waitProgressDone(client)
		}
		return nil

	case "saves":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(reorderFlags(fs, args)); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 3 {
			return fmt.Errorf("usage: hikari saves <store> <id> download|upload")
		}
		var res map[string]any
		if err := dial(c).Call("game.saves.sync", map[string]any{
			"store": pos[0], "id": pos[1], "direction": pos[2],
		}, &res); err != nil {
			return err
		}
		if c.json {
			return c.print(res)
		}
		if out, _ := res["output"].(string); out != "" {
			fmt.Print(out)
		}
		return nil

	case "wines":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		var res struct {
			Versions []struct {
				Name   string `json:"name"`
				Bin    string `json:"bin"`
				Type   string `json:"type"`
				Source string `json:"source"`
			} `json:"versions"`
		}
		if err := dial(c).Call("wine.versions", nil, &res); err != nil {
			return err
		}
		if c.json {
			return c.print(res)
		}
		for _, v := range res.Versions {
			fmt.Printf("%-10s %-34s %-8s %s\n", v.Source, v.Name, v.Type, v.Bin)
		}
		return nil

	case "wine":
		return wineCmd(args)

	case "engines":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		var res struct {
			Engines map[string]engineView `json:"engines"`
		}
		if err := dial(c).Call("engine.versions", nil, &res); err != nil {
			return err
		}
		if c.json {
			return c.print(res)
		}
		names := make([]string, 0, len(res.Engines))
		for name := range res.Engines {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			e := res.Engines[name]
			state := "missing"
			if e.Available {
				state = orDefault(e.Version, e.Path)
			}
			fmt.Printf("%-10s %s\n", name, state)
		}
		return nil

	case "mods":
		return modsCmd(args)

	case "shutdown":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args); err != nil {
			return err
		}
		if err := dial(c).Call("daemon.stop", nil, nil); err != nil {
			return err
		}
		fmt.Println("hikarid stopping")
		return nil

	default:
		return fmt.Errorf("unknown command %q (see hikari help)", cmd)
	}
}

func wineCmd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: hikari wine releases|install <tag>|remove <name>")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "releases":
		fs := flag.NewFlagSet("wine releases", flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(rest); err != nil {
			return err
		}
		var res struct {
			Releases []struct {
				Tag     string `json:"tag"`
				Variant string `json:"variant"`
				Date    string `json:"date"`
				Size    int64  `json:"size"`
			} `json:"releases"`
		}
		if err := dial(c).Call("wine.releases", nil, &res); err != nil {
			return err
		}
		if c.json {
			return c.print(res)
		}
		for _, r := range res.Releases {
			fmt.Printf("%-20s %-18s %s  %4d MiB\n", r.Tag, r.Variant, r.Date, r.Size/1024/1024)
		}
		return nil
	case "install":
		fs := flag.NewFlagSet("wine install", flag.ExitOnError)
		c := addCommon(fs)
		follow := fs.Bool("follow", false, "stream download progress")
		if err := fs.Parse(reorderFlags(fs, rest)); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 1 {
			return fmt.Errorf("usage: hikari wine install <tag> [--follow]")
		}
		client := dial(c)
		if *follow {
			client.OnEvent = func(method string, params json.RawMessage) {
				if method != "wine.progress" {
					return
				}
				var p struct {
					Phase   string   `json:"phase"`
					Percent *float64 `json:"percent"`
					Name    string   `json:"name"`
					Done    bool     `json:"done"`
					Error   string   `json:"error"`
				}
				if json.Unmarshal(params, &p) != nil {
					return
				}
				pct := "-"
				if p.Percent != nil {
					pct = strconv.FormatFloat(*p.Percent, 'f', 1, 64) + "%"
				}
				fmt.Printf("\r%-12s %7s %s   ", p.Phase, pct, p.Name)
				if p.Done {
					fmt.Println()
					if p.Error != "" {
						fmt.Println("error:", p.Error)
					}
				}
			}
			if err := client.Subscribe("wine.progress"); err != nil {
				return err
			}
		}
		var res map[string]any
		if err := client.Call("wine.download", map[string]any{"tag": pos[0]}, &res); err != nil {
			return err
		}
		fmt.Println("download started:", res["job"])
		if *follow {
			for {
				params, err := client.AwaitEvent("wine.progress", 24*time.Hour)
				if err != nil {
					return nil
				}
				var p struct {
					Done bool `json:"done"`
				}
				if json.Unmarshal(params, &p) == nil && p.Done {
					return nil
				}
			}
		}
		return nil
	case "remove":
		fs := flag.NewFlagSet("wine remove", flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(rest); err != nil {
			return err
		}
		pos := fs.Args()
		if len(pos) < 1 {
			return fmt.Errorf("usage: hikari wine remove <name>")
		}
		if err := dial(c).Call("wine.remove", map[string]any{"name": pos[0]}, nil); err != nil {
			return err
		}
		fmt.Println("removed", pos[0])
		return nil
	default:
		return fmt.Errorf("unknown wine subcommand %q", sub)
	}
}

func authCmd(args []string) error {
	fs := flag.NewFlagSet("auth", flag.ExitOnError)
	c := addCommon(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	pos := fs.Args()
	if len(pos) == 0 {
		return fmt.Errorf("usage: hikari auth status|login|logout ...")
	}
	client := dial(c)
	switch pos[0] {
	case "status":
		var res map[string]any
		if err := client.Call("auth.status", nil, &res); err != nil {
			return err
		}
		return c.print(res)
	case "login":
		if len(pos) < 2 {
			return fmt.Errorf("usage: hikari auth login <store> [code]")
		}
		params := map[string]any{"store": pos[1]}
		if len(pos) > 2 {
			params["code"] = pos[2]
		}
		var res map[string]any
		if err := client.Call("auth.login", params, &res); err != nil {
			return err
		}
		if state, _ := res["state"].(string); state != "pending" {
			return c.print(res)
		}
		fmt.Println("open this URL in a browser and log in:")
		fmt.Println(" ", res["url"])
		fmt.Println(res["hint"])
		fmt.Print("paste the code here: ")
		code, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		code = strings.TrimSpace(code)
		if code == "" {
			return fmt.Errorf("no code given; login aborted (run again to restart)")
		}
		var done map[string]any
		if err := client.Call("auth.login", map[string]any{"store": pos[1], "code": code}, &done); err != nil {
			return err
		}
		return c.print(done)
	case "logout":
		if len(pos) < 2 {
			return fmt.Errorf("usage: hikari auth logout <store>")
		}
		if err := client.Call("auth.logout", map[string]any{"store": pos[1]}, nil); err != nil {
			return err
		}
		fmt.Println("logged out")
		return nil
	default:
		return fmt.Errorf("unknown auth subcommand %q", pos[0])
	}
}

type gameView struct {
	Store       string `json:"store"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Installed   bool   `json:"installed"`
	InstallPath string `json:"install_path"`
	Platform    string `json:"platform"`
	Source      string `json:"source"`
}

type engineView struct {
	Available bool   `json:"available"`
	Path      string `json:"path"`
	Version   string `json:"version"`
}

type statusView struct {
	Version       string                `json:"version"`
	Socket        string                `json:"socket"`
	UptimeSeconds int64                 `json:"uptime_seconds"`
	Engines       map[string]engineView `json:"engines"`
	Stores        []statusStoreView     `json:"stores"`
}

type statusStoreView struct {
	Store     string `json:"store"`
	Available bool   `json:"available"`
	LoggedIn  bool   `json:"logged_in"`
	User      string `json:"user"`
}

func statusText(s statusView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "hikari %s, up %s\n", orDefault(s.Version, "?"), humanUptime(s.UptimeSeconds))
	if s.Socket != "" {
		fmt.Fprintf(&b, "socket  %s\n", s.Socket)
	}
	if len(s.Engines) > 0 {
		names := make([]string, 0, len(s.Engines))
		for name := range s.Engines {
			names = append(names, name)
		}
		sort.Strings(names)
		parts := make([]string, 0, len(names))
		for _, name := range names {
			if s.Engines[name].Available {
				parts = append(parts, name)
			} else {
				parts = append(parts, name+" (missing)")
			}
		}
		fmt.Fprintf(&b, "engines %s\n", strings.Join(parts, ", "))
	}
	if len(s.Stores) > 0 {
		parts := make([]string, 0, len(s.Stores))
		for _, st := range s.Stores {
			switch {
			case st.User != "":
				parts = append(parts, st.Store+" ("+st.User+")")
			case hasLogin(st.Store):
				parts = append(parts, st.Store+" (logged out)")
			default:
				parts = append(parts, st.Store)
			}
		}
		fmt.Fprintf(&b, "stores  %s\n", strings.Join(parts, ", "))
	}
	return b.String()
}

func hasLogin(store string) bool {
	switch store {
	case "epic", "gog", "amazon":
		return true
	}
	return false
}

func humanUptime(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	switch {
	case d < time.Minute:
		return strconv.FormatInt(int64(d.Seconds()), 10) + "s"
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

type planView struct {
	Argv     []string          `json:"argv"`
	Env      map[string]string `json:"env"`
	Cwd      string            `json:"cwd"`
	Engine   string            `json:"engine"`
	WineType string            `json:"wine_type"`
}

func printPlan(p *planView) {
	fmt.Println("engine:   ", p.Engine)
	fmt.Println("wine:     ", p.WineType)
	fmt.Println("directory:", orDefault(p.Cwd, "(inherit)"))
	fmt.Println("command:")
	fmt.Println("  " + shellLine(p.Argv))
	fmt.Println("environment:")
	fmt.Println(envDiff(p.Env))
	fmt.Println()
	fmt.Println("run it yourself:")
	parts := make([]string, 0, len(p.Env)+len(p.Argv))
	for k, v := range p.Env {
		parts = append(parts, k+"="+shellQuote(v))
	}
	for _, a := range p.Argv {
		parts = append(parts, shellQuote(a))
	}
	fmt.Println("  " + strings.Join(parts, " "))
}

func envDiff(env map[string]string) string {
	if len(env) == 0 {
		return "  (none)"
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s=%s\n", k, env[k])
	}
	return strings.TrimRight(b.String(), "\n")
}

func watchLaunch(client *api.Client, store, id string) error {
	deadline := time.Now().Add(3 * time.Second)
	for {
		var rec struct {
			Running  bool     `json:"running"`
			State    string   `json:"state"`
			Error    string   `json:"error"`
			ExitCode int      `json:"exit_code"`
			Lines    []string `json:"lines"`
		}
		if err := client.Call("game.log", map[string]any{"store": store, "id": id}, &rec); err != nil {
			return nil
		}
		switch rec.State {
		case "playing":
			return nil
		case "crashed":
			msg := rec.Error
			if msg == "" {
				msg = "the game exited during startup"
			}
			fmt.Fprintf(os.Stderr, "%s/%s crashed: %s", store, id, msg)
			if rec.ExitCode != 0 {
				fmt.Fprintf(os.Stderr, " (exit %d)", rec.ExitCode)
			}
			fmt.Fprintln(os.Stderr)
			for _, line := range rec.Lines {
				fmt.Fprintln(os.Stderr, "  "+line)
			}
			fmt.Fprintln(os.Stderr, "run `hikari logs --game "+store+" "+id+"` for the full output")
			return fmt.Errorf("launch failed")
		case "stopped":
			return nil
		}
		if !rec.Running || time.Now().After(deadline) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func progressPrinter() func(string, json.RawMessage) {
	return func(method string, params json.RawMessage) {
		if method != "install.progress" {
			return
		}
		var p struct {
			Store   string   `json:"store"`
			ID      string   `json:"id"`
			State   string   `json:"state"`
			Percent *float64 `json:"percent"`
			Bytes   string   `json:"bytes"`
			ETA     string   `json:"eta"`
		}
		if json.Unmarshal(params, &p) != nil {
			return
		}
		pct := "-"
		if p.Percent != nil {
			pct = strconv.FormatFloat(*p.Percent, 'f', 1, 64) + "%"
		}
		fmt.Printf("\r%s/%s: %-11s %7s %10s eta %-8s   ",
			p.Store, p.ID, p.State, pct, orDefault(p.Bytes, "-"), orDefault(p.ETA, "-"))
		if p.State == "done" || p.State == "error" || p.State == "canceled" {
			fmt.Println()
		}
	}
}

func waitProgressDone(client *api.Client) {
	for {
		params, err := client.AwaitEvent("install.progress", 24*time.Hour)
		if err != nil {
			return
		}
		var p struct {
			State string `json:"state"`
		}
		if json.Unmarshal(params, &p) == nil {
			switch p.State {
			case "done", "error", "canceled":
				return
			}
		}
	}
}

func readSettingValue(v string) ([]byte, error) {
	switch {
	case v == "-":
		return io.ReadAll(bufio.NewReader(os.Stdin))
	case strings.HasPrefix(v, "@"):
		return os.ReadFile(strings.TrimPrefix(v, "@"))
	default:
		return []byte(v), nil
	}
}

func mergeSettingJSON(base map[string]any, patch []byte) ([]byte, error) {
	baseRaw, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(baseRaw, &merged); err != nil {
		return nil, err
	}
	var p map[string]json.RawMessage
	if err := json.Unmarshal(patch, &p); err != nil {
		return nil, fmt.Errorf("settings must be a JSON object: %w", err)
	}
	for key, value := range p {
		merged[key] = value
	}
	return json.Marshal(merged)
}

func shellQuote(s string) string {
	if strings.ContainsAny(s, " \t\"'") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

func shellLine(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		if strings.ContainsAny(a, " \t\"'") {
			parts[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// reorderFlags puts flags before positionals, reading arity from the FlagSet.
func reorderFlags(fs *flag.FlagSet, args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" || a == "--" {
			pos = append(pos, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue
		}
		f := fs.Lookup(name)
		if f == nil {
			continue
		}
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, pos...)
}
