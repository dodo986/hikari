package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const modsJobTimeout = 24 * time.Hour

func modsCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: hikari mods status|games|setup|login|logout|account|deploy <game>|restore <game>|install <game> (--archive PATH | --url nxm://… | --mod-id N --file-id N)|browse <game> [query]|files <game> <mod-id>|installed <game>")
	}
	switch args[0] {
	case "status":
		return modsStatusCmd(args[1:])
	case "games":
		return modsGamesCmd(args[1:])
	case "setup":
		return modsJobCmd("mods.setup", "setup", nil, args[1:], func(*flag.FlagSet, []string) (map[string]any, error) {
			return map[string]any{}, nil
		})
	case "nexus":
		return modsNexusCmd(args[1:])
	case "login":
		return modsJobCmd("mods.login", "login", nil, args[1:], func(*flag.FlagSet, []string) (map[string]any, error) {
			return map[string]any{}, nil
		})
	case "logout":
		return modsLogoutCmd(args[1:])
	case "account":
		return modsAccountCmd(args[1:])
	case "browse":
		return modsBrowseCmd(args[1:])
	case "files":
		return modsFilesCmd(args[1:])
	case "installed":
		return modsInstalledCmd(args[1:])
	case "cancel":
		fs := flag.NewFlagSet("mods cancel", flag.ExitOnError)
		c := addCommon(fs)
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if err := dial(c).Call("mods.cancel", nil, nil); err != nil {
			return err
		}
		fmt.Println("cancel: the running job was stopped")
		return nil
	case "deploy":
		return modsJobCmd("mods.deploy", "deploy", []string{"profile"}, args[1:], func(fs *flag.FlagSet, pos []string) (map[string]any, error) {
			if len(pos) < 1 {
				return nil, errors.New("usage: hikari mods deploy <game> [--profile NAME]")
			}
			params := map[string]any{"game": pos[0]}
			copyFlags(fs, params, "profile")
			return params, nil
		})
	case "restore":
		return modsJobCmd("mods.restore", "restore", nil, args[1:], func(_ *flag.FlagSet, pos []string) (map[string]any, error) {
			if len(pos) < 1 {
				return nil, errors.New("usage: hikari mods restore <game>")
			}
			return map[string]any{"game": pos[0]}, nil
		})
	case "install":
		return modsJobCmd("mods.install", "install", []string{"profile", "archive", "url", "name", "mod-id", "file-id"}, args[1:], func(fs *flag.FlagSet, pos []string) (map[string]any, error) {
			if len(pos) < 1 {
				return nil, errors.New("usage: hikari mods install <game> (--archive PATH | --url nxm://… | --mod-id N --file-id N) [--profile NAME] [--name NAME]")
			}
			params := map[string]any{"game": pos[0]}
			copyFlags(fs, params, "profile", "archive", "url", "name")
			if params["mod-id"] != nil || params["file-id"] != nil {
				if params["mod-id"] == nil || params["file-id"] == nil {
					return nil, errors.New("--mod-id and --file-id go together")
				}
				return params, nil
			}
			if params["archive"] == nil && params["url"] == nil {
				return nil, errors.New("pass --archive PATH, --url nxm://… or --mod-id N --file-id N to install")
			}
			return params, nil
		})
	}
	return fmt.Errorf("unknown mods command %q", args[0])
}

func modsAccountCmd(args []string) error {
	fs := flag.NewFlagSet("mods account", flag.ExitOnError)
	c := addCommon(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	var res struct {
		Stored  bool   `json:"stored"`
		Mode    string `json:"mode"`
		User    string `json:"user"`
		Premium bool   `json:"premium"`
	}
	if err := dial(c).Call("mods.account", nil, &res); err != nil {
		return err
	}
	if c.json {
		return c.print(res)
	}
	if !res.Stored {
		fmt.Println("Nexus: not signed in (run `hikari mods login`)")
		return nil
	}
	plan := "free (browsing only, use Mod manager download on the site)"
	if res.Premium {
		plan = "premium (direct downloads)"
	}
	how := "browser login"
	if res.Mode == "key" {
		how = "API key"
	}
	fmt.Printf("Nexus: %s — %s, by %s\n", res.User, plan, how)
	return nil
}

func modsLogoutCmd(args []string) error {
	fs := flag.NewFlagSet("mods logout", flag.ExitOnError)
	c := addCommon(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := dial(c).Call("mods.logout", nil, nil); err != nil {
		return err
	}
	fmt.Println("Nexus: signed out (the stored browser login was forgotten)")
	return nil
}

func modsBrowseCmd(args []string) error {
	fs := flag.NewFlagSet("mods browse", flag.ExitOnError)
	c := addCommon(fs)
	sort := fs.String("sort", "", "")
	page := fs.Int("page", 0, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	pos := fs.Args()
	if len(pos) < 1 {
		return errors.New("usage: hikari mods browse <game> [query] [--sort downloads|endorsements|created|updated] [--page N]")
	}
	params := map[string]any{"game": pos[0], "sort": *sort, "page": *page}
	if len(pos) > 1 {
		params["query"] = strings.Join(pos[1:], " ")
	}
	var res struct {
		Game    string `json:"game"`
		Domain  string `json:"domain"`
		Page    int    `json:"page"`
		More    bool   `json:"more"`
		Results []struct {
			ModID        int    `json:"mod_id"`
			Name         string `json:"name"`
			Author       string `json:"author"`
			Version      string `json:"version"`
			Downloads    int    `json:"downloads"`
			Endorsements int    `json:"endorsements"`
			Summary      string `json:"summary"`
		} `json:"results"`
	}
	if err := dial(c).Call("mods.browse", params, &res); err != nil {
		return err
	}
	if c.json {
		return c.print(res)
	}
	for _, mod := range res.Results {
		fmt.Printf("%-7d %-44.44s %-22.22s %8d dl %7d end  %s\n",
			mod.ModID, mod.Name, mod.Author, mod.Downloads, mod.Endorsements, mod.Version)
	}
	if res.More {
		fmt.Printf("page %d done — add --page %d for more\n", res.Page+1, res.Page+1)
	}
	return nil
}

func modsFilesCmd(args []string) error {
	fs := flag.NewFlagSet("mods files", flag.ExitOnError)
	c := addCommon(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	pos := fs.Args()
	if len(pos) < 2 {
		return errors.New("usage: hikari mods files <game> <mod-id>")
	}
	modID, err := strconv.Atoi(pos[1])
	if err != nil {
		return fmt.Errorf("mod id %q is not a number", pos[1])
	}
	var res struct {
		Mod struct {
			ModID int    `json:"mod_id"`
			Name  string `json:"name"`
		} `json:"mod"`
		Files []struct {
			FileID   int    `json:"file_id"`
			Name     string `json:"name"`
			Version  string `json:"version"`
			Category string `json:"category"`
			Size     string `json:"size"`
			Primary  bool   `json:"primary"`
		} `json:"files"`
	}
	if err := dial(c).Call("mods.files", map[string]any{"game": pos[0], "mod_id": modID}, &res); err != nil {
		return err
	}
	if c.json {
		return c.print(res)
	}
	fmt.Printf("%s (%d)\n", res.Mod.Name, res.Mod.ModID)
	for _, file := range res.Files {
		marker := " "
		if file.Primary {
			marker = "*"
		}
		fmt.Printf("%s %-8d %-44.44s %-12s %-10s %s\n", marker, file.FileID, file.Name, file.Version, file.Category, file.Size)
	}
	return nil
}

func modsInstalledCmd(args []string) error {
	fs := flag.NewFlagSet("mods installed", flag.ExitOnError)
	c := addCommon(fs)
	profile := fs.String("profile", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	pos := fs.Args()
	if len(pos) < 1 {
		return errors.New("usage: hikari mods installed <game> [--profile NAME]")
	}
	var res struct {
		Game    string `json:"game"`
		Profile string `json:"profile"`
		Mods    []struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"mods"`
	}
	if err := dial(c).Call("mods.installed", map[string]any{"game": pos[0], "profile": *profile}, &res); err != nil {
		return err
	}
	if c.json {
		return c.print(res)
	}
	fmt.Printf("%s / %s — %d mod(s)\n", res.Game, res.Profile, len(res.Mods))
	for _, mod := range res.Mods {
		marker := "[x]"
		if !mod.Enabled {
			marker = "[ ]"
		}
		fmt.Printf("%s %s\n", marker, mod.Name)
	}
	return nil
}

func modsNexusCmd(args []string) error {
	action := "status"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("mods nexus", flag.ExitOnError)
	c := addCommon(fs)
	key := fs.String("key", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	params := map[string]any{"action": action}
	switch action {
	case "status":
	case "set":
		value := *key
		if value == "-" || value == "" {
			line, err := readSecretLine()
			if err != nil {
				return err
			}
			value = line
		}
		params["key"] = value
	case "clear":
	default:
		return fmt.Errorf("unknown nexus action %q (use status, set or clear)", action)
	}

	var res struct {
		Stored bool `json:"stored"`
	}
	if err := dial(c).Call("mods.nexus", params, &res); err != nil {
		return err
	}
	if c.json {
		return c.print(res)
	}
	if res.Stored {
		fmt.Println("Nexus API key: stored, where Amethyst keeps it")
	} else {
		fmt.Println("Nexus API key: not set — add one at nexusmods.com (Account ▸ API) and run `hikari mods nexus set`")
	}
	return nil
}

func readSecretLine() (string, error) {
	fmt.Fprint(os.Stderr, "Nexus API key: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("no API key was read from stdin")
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return "", errors.New("an empty API key changes nothing")
	}
	return value, nil
}

func modsStatusCmd(args []string) error {
	fs := flag.NewFlagSet("mods status", flag.ExitOnError)
	c := addCommon(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	var status struct {
		Available bool   `json:"available"`
		Python    string `json:"python"`
		Source    string `json:"src"`
		Reason    string `json:"reason"`
	}
	if err := dial(c).Call("mods.status", nil, &status); err != nil {
		return err
	}
	if c.json {
		return c.print(status)
	}
	if status.Available {
		fmt.Println("Amethyst backend: ready")
	} else if status.Reason != "" {
		fmt.Println("Amethyst backend: unavailable —", status.Reason)
	} else {
		fmt.Println("Amethyst backend: unavailable")
	}
	if status.Python != "" {
		fmt.Println("python:", status.Python)
	}
	if status.Source != "" {
		fmt.Println("source:", status.Source)
	}
	return nil
}

func modsGamesCmd(args []string) error {
	fs := flag.NewFlagSet("mods games", flag.ExitOnError)
	c := addCommon(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	var res struct {
		Games []struct {
			ID              string   `json:"id"`
			Name            string   `json:"name"`
			Configured      bool     `json:"configured"`
			DeployActive    bool     `json:"deploy_active"`
			DeployedProfile string   `json:"deployed_profile"`
			Store           string   `json:"store"`
			GameID          string   `json:"game_id"`
			Profiles        []string `json:"profiles"`
		} `json:"games"`
	}
	if err := dial(c).Call("mods.games", nil, &res); err != nil {
		return err
	}
	if c.json {
		return c.print(res)
	}
	for _, game := range res.Games {
		state := "not configured"
		if game.Configured {
			state = "ready"
		}
		deployed := "-"
		if game.DeployActive {
			deployed = game.DeployedProfile
		}
		library := "-"
		if game.Store != "" {
			library = game.Store + "/" + game.GameID
		}
		fmt.Printf("%-34s %-20s %-14s %-12s %s\n", game.Name, game.ID, state, deployed, library)
	}
	return nil
}

func modsJobCmd(method, label string, names []string, args []string, build func(*flag.FlagSet, []string) (map[string]any, error)) error {
	fs := flag.NewFlagSet("mods "+label, flag.ExitOnError)
	c := addCommon(fs)
	for _, name := range names {
		fs.String(name, "", name)
	}
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		return err
	}
	params, err := build(fs, fs.Args())
	if err != nil {
		return err
	}

	client := dial(c)
	var done map[string]any
	printed := false
	client.OnEvent = func(event string, payload json.RawMessage) {
		if event == "mods.done" {
			_ = json.Unmarshal(payload, &done)
			return
		}
		if c.json {
			return
		}
		switch event {
		case "mods.auth_url":
			var p struct {
				URL string `json:"url"`
			}
			if json.Unmarshal(payload, &p) == nil && p.URL != "" {
				if printed {
					fmt.Println()
					printed = false
				}
				fmt.Println("  authorise Hikari here: " + p.URL)
				openBrowser(p.URL)
			}
		case "mods.log":
			var p struct {
				Line string `json:"line"`
			}
			if json.Unmarshal(payload, &p) == nil && strings.TrimSpace(p.Line) != "" {
				if printed {
					fmt.Println()
					printed = false
				}
				fmt.Println("  " + p.Line)
			}
		case "mods.progress":
			var p struct {
				Phase   string `json:"phase"`
				Current int    `json:"current"`
				Total   int    `json:"total"`
				Message string `json:"message"`
			}
			if json.Unmarshal(payload, &p) == nil && p.Total > 0 {
				fmt.Printf("\r  %s %d/%d %s   ", p.Phase, p.Current, p.Total, p.Message)
				printed = true
			}
		}
	}
	if err := client.Subscribe("mods.log", "mods.progress", "mods.done", "mods.auth_url"); err != nil {
		return err
	}

	var res struct {
		Job string `json:"job"`
	}
	if err := client.Call(method, params, &res); err != nil {
		return err
	}
	for done == nil {
		if _, err := client.AwaitEvent("mods.done", modsJobTimeout); err != nil {
			return err
		}
	}
	if c.json {
		return c.print(done)
	}
	if printed {
		fmt.Println()
	}
	if ok, _ := done["ok"].(bool); !ok {
		message, _ := done["error"].(string)
		if message == "" {
			message = "the mods job failed"
		}
		return errors.New(message)
	}
	if mod, _ := done["mod"].(string); mod != "" {
		fmt.Println(label + ": " + mod)
		return nil
	}
	fmt.Println(label + ": ok")
	return nil
}

func copyFlags(fs *flag.FlagSet, params map[string]any, names ...string) {
	for _, name := range names {
		if value := fs.Lookup(name); value != nil && value.Value.String() != "" {
			params[name] = value.Value.String()
		}
	}
}

// openBrowser shows the Nexus authorisation page; the printed URL stays the fallback.
func openBrowser(url string) {
	for _, opener := range []string{"xdg-open", "gio"} {
		path, err := exec.LookPath(opener)
		if err != nil {
			continue
		}
		args := []string{url}
		if opener == "gio" {
			args = []string{"open", url}
		}
		cmd := exec.Command(path, args...)
		cmd.Stdout, cmd.Stderr = nil, nil
		if cmd.Start() == nil {
			_ = cmd.Process.Release()
			return
		}
	}
}
