package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hikari/internal/mods"
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
	case "nxm":
		return modsNxmCmd(args[1:])
	case "remove":
		return modsRemoveCmd(args[1:])
	case "toggle":
		return modsToggleCmd(args[1:])
	case "missing":
		return modsMissingCmd(args[1:])
	case "overrides":
		return modsOverridesCmd(args[1:])
	case "downloads":
		return modsDownloadsCmd(args[1:])
	case "scheme":
		return modsSchemeCmd(args[1:])
	case "install":
		return modsJobCmd("mods.install", "install", []string{"profile", "archive", "url", "name", "mod-id", "file-id"}, args[1:], installParams)
	}
	return fmt.Errorf("unknown mods command %q", args[0])
}

// modsNxmCmd is what a browser runs: nxm://<domain>/mods/<id>/files/<id>?key=…
// The domain has to become one of Amethyst's game keys before anything else.
func modsNxmCmd(args []string) error {
	fs := flag.NewFlagSet("mods nxm", flag.ExitOnError)
	c := addCommon(fs)
	profile := fs.String("profile", "", "")
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		return err
	}
	pos := fs.Args()
	if len(pos) < 1 {
		return errors.New("usage: hikari mods nxm nxm://<game>/mods/<id>/files/<id>?key=…")
	}
	link := pos[0]
	domain := nxmDomain(link)
	if domain == "" {
		return fmt.Errorf("%q is not an nxm:// mod link", link)
	}

	client := dial(c)
	var games struct {
		Games []mods.Game `json:"games"`
	}
	if err := client.Call("mods.games", nil, &games); err != nil {
		return err
	}
	game := ""
	for _, known := range games.Games {
		if strings.EqualFold(known.NexusDomain, domain) {
			game = known.ID
			break
		}
	}
	if game == "" {
		return fmt.Errorf("Amethyst has no game handler for %q; set the game up in Amethyst first", domain)
	}
	params := map[string]any{"game": game, "url": link}
	if *profile != "" {
		params["profile"] = *profile
	}
	return runModsJob(c, "mods.install", "installed", params)
}

// nxmDomain pulls the game domain out of an nxm link.
func nxmDomain(link string) string {
	rest, ok := strings.CutPrefix(link, "nxm://")
	if !ok {
		return ""
	}
	domain, _, found := strings.Cut(rest, "/")
	if !found || domain == "" {
		return ""
	}
	return domain
}

const nxmDesktop = "hikari-nxm.desktop"

// modsRemoveCmd deletes one mod: files, plugins, staging folder and modlist row.
func modsRemoveCmd(args []string) error {
	fs := flag.NewFlagSet("mods remove", flag.ExitOnError)
	c := addCommon(fs)
	profile := fs.String("profile", "", "")
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		return err
	}
	pos := fs.Args()
	if len(pos) < 2 {
		return errors.New("usage: hikari mods remove <game> <mod name>")
	}
	params := map[string]any{"game": pos[0], "name": strings.Join(pos[1:], " ")}
	if *profile != "" {
		params["profile"] = *profile
	}
	return runModsJob(c, "mods.remove", "removed", params)
}

// modsToggleCmd enables or disables one modlist row.
func modsToggleCmd(args []string) error {
	fs := flag.NewFlagSet("mods toggle", flag.ExitOnError)
	c := addCommon(fs)
	profile := fs.String("profile", "", "")
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		return err
	}
	pos := fs.Args()
	if len(pos) < 2 {
		return errors.New("usage: hikari mods toggle <game> <mod name>")
	}
	params := map[string]any{"game": pos[0], "name": strings.Join(pos[1:], " ")}
	if *profile != "" {
		params["profile"] = *profile
	}
	var out struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := dial(c).Call("mods.toggle", params, &out); err != nil {
		return err
	}
	if c.json {
		return c.print(out)
	}
	state := "disabled"
	if out.Enabled {
		state = "enabled"
	}
	fmt.Printf("%s is %s; deploy to apply it\n", out.Name, state)
	return nil
}

// modsMissingCmd lists Nexus requirements the installed mods are missing.
func modsMissingCmd(args []string) error {
	fs := flag.NewFlagSet("mods missing", flag.ExitOnError)
	c := addCommon(fs)
	profile := fs.String("profile", "", "")
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		return err
	}
	pos := fs.Args()
	if len(pos) < 1 {
		return errors.New("usage: hikari mods missing <game> [--profile NAME]")
	}
	params := map[string]any{"game": pos[0]}
	if *profile != "" {
		params["profile"] = *profile
	}
	var out struct {
		Missing []mods.Missing `json:"missing"`
	}
	if err := dial(c).Call("mods.missing", params, &out); err != nil {
		return err
	}
	if c.json {
		return c.print(out)
	}
	if len(out.Missing) == 0 {
		fmt.Println("every listed requirement is installed")
		return nil
	}
	for _, item := range out.Missing {
		for _, need := range item.Needs {
			name := need.Name
			if need.External {
				name += " (outside Nexus)"
			}
			if need.ModID > 0 {
				fmt.Printf("%s needs %s [mod %d]\n", item.Mod, name, need.ModID)
			} else {
				fmt.Printf("%s needs %s\n", item.Mod, name)
			}
		}
	}
	return nil
}

// modsOverridesCmd prints the Wine DLL overrides a game's mods need.
func modsOverridesCmd(args []string) error {
	fs := flag.NewFlagSet("mods overrides", flag.ExitOnError)
	c := addCommon(fs)
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		return err
	}
	pos := fs.Args()
	if len(pos) < 1 {
		return errors.New("usage: hikari mods overrides <game>")
	}
	var out struct {
		Overrides map[string]string `json:"overrides"`
	}
	if err := dial(c).Call("mods.overrides", map[string]any{"game": pos[0]}, &out); err != nil {
		return err
	}
	if c.json {
		return c.print(out)
	}
	if len(out.Overrides) == 0 {
		fmt.Println("this game's mods need no DLL overrides")
		return nil
	}
	names := make([]string, 0, len(out.Overrides))
	for name := range out.Overrides {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Printf("%s=%s\n", name, out.Overrides[name])
	}
	return nil
}

// modsDownloadsCmd answers "has the browser download finished?" so a script
// can wait for it and then install the archive.
func modsDownloadsCmd(args []string) error {
	fs := flag.NewFlagSet("mods downloads", flag.ExitOnError)
	c := addCommon(fs)
	modID := fs.Int("mod-id", 0, "")
	fileID := fs.Int("file-id", 0, "")
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		return err
	}
	pos := fs.Args()
	if len(pos) < 1 || *modID <= 0 {
		return errors.New("usage: hikari mods downloads <game> --mod-id N [--file-id N]")
	}
	params := map[string]any{"game": pos[0], "mod_id": *modID}
	if *fileID > 0 {
		params["file_id"] = *fileID
	}
	var out struct {
		Found string `json:"found"`
	}
	if err := dial(c).Call("mods.downloads", params, &out); err != nil {
		return err
	}
	if c.json {
		return c.print(out)
	}
	if out.Found == "" {
		fmt.Println("downloads: nothing yet")
		return nil
	}
	fmt.Println(out.Found)
	return nil
}

// modsSchemeCmd points the desktop's nxm:// handler at this binary.
func modsSchemeCmd(args []string) error {
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}
	switch action {
	case "status":
		fmt.Println("nxm:// opens: " + currentNxmHandler())
		return nil
	case "set":
		binary, err := os.Executable()
		if err != nil {
			return fmt.Errorf("cannot find my own path: %w", err)
		}
		dir := filepath.Join(xdgDataHome(), "applications")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		// The launcher wrapper opens the window so the download is visible.
		command := binary + " mods nxm %u"
		if app := filepath.Join(filepath.Dir(binary), "hikari-app"); fileExists(app) {
			command = app + " nxm %u"
		}
		entry := "[Desktop Entry]\n" +
			"Type=Application\n" +
			"Name=Hikari (Nexus links)\n" +
			"Comment=Install nxm:// mod downloads with Hikari\n" +
			"Exec=" + command + "\n" +
			"MimeType=x-scheme-handler/nxm;\n" +
			"Terminal=false\n" +
			"NoDisplay=true\n" +
			"Categories=Game;\n"
		if err := os.WriteFile(filepath.Join(dir, nxmDesktop), []byte(entry), 0o644); err != nil {
			return err
		}
		if err := exec.Command("update-desktop-database", dir).Run(); err != nil {
			// Not fatal: some systems have no update-desktop-database.
			fmt.Fprintln(os.Stderr, "hikari: desktop database not refreshed:", err)
		}
		if err := exec.Command("xdg-mime", "default", nxmDesktop, "x-scheme-handler/nxm").Run(); err != nil {
			return fmt.Errorf("wrote %s but could not make it the default: %w", nxmDesktop, err)
		}
		fmt.Println("nxm:// now opens: " + currentNxmHandler())
		return nil
	}
	return fmt.Errorf("usage: hikari mods scheme [status|set]")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func currentNxmHandler() string {
	out, err := exec.Command("xdg-mime", "query", "default", "x-scheme-handler/nxm").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "nothing registered"
	}
	return strings.TrimSpace(string(out))
}

func xdgDataHome() string {
	if value := os.Getenv("XDG_DATA_HOME"); value != "" {
		return value
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

func modsAccountCmd(args []string) error {
	fs := flag.NewFlagSet("mods account", flag.ExitOnError)
	c := addCommon(fs)
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
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
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
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
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
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
	var res mods.Browse
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
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
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
	var res mods.Files
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
		uploaded := ""
		if date, err := time.Parse(time.RFC3339, file.Uploaded); err == nil {
			uploaded = date.Format("2006-01-02")
		}
		fmt.Printf("%s %-8d %-40.40s %-12s %-14s %-10s %s\n",
			marker, file.FileID, file.Name, file.Version, file.Category, file.Size, uploaded)
	}
	return nil
}

func modsInstalledCmd(args []string) error {
	fs := flag.NewFlagSet("mods installed", flag.ExitOnError)
	c := addCommon(fs)
	profile := fs.String("profile", "", "")
	// Flags after the game name are common ("installed <game> --json").
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
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
	return runModsJob(c, method, label, params)
}

// runModsJob starts one daemon job and streams it until mods.done.
func runModsJob(c *commonFlags, method, label string, params map[string]any) error {
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

// installParams turns the flags into the daemon's params: the wire speaks
// snake_case, and ids travel as numbers, so both are translated here.
func installParams(fs *flag.FlagSet, pos []string) (map[string]any, error) {
	if len(pos) < 1 {
		return nil, errors.New("usage: hikari mods install <game> (--archive PATH | --url nxm://… | --mod-id N --file-id N) [--profile NAME] [--name NAME]")
	}
	params := map[string]any{"game": pos[0]}
	copyFlags(fs, params, "profile", "archive", "url", "name")
	modID, fileID := fs.Lookup("mod-id").Value.String(), fs.Lookup("file-id").Value.String()
	if modID == "" && fileID == "" {
		if params["archive"] == nil && params["url"] == nil {
			return nil, errors.New("pass --archive PATH, --url nxm://… or --mod-id N --file-id N to install")
		}
		return params, nil
	}
	if modID == "" || fileID == "" {
		return nil, errors.New("--mod-id and --file-id go together")
	}
	id, err := strconv.Atoi(modID)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("--mod-id %q is not a Nexus mod id", modID)
	}
	file, err := strconv.Atoi(fileID)
	if err != nil || file <= 0 {
		return nil, fmt.Errorf("--file-id %q is not a Nexus file id", fileID)
	}
	params["mod_id"], params["file_id"] = id, file
	return params, nil
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
