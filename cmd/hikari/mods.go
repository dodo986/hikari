package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"
)

const modsJobTimeout = 24 * time.Hour

func modsCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: hikari mods status|games|setup|deploy <game>|restore <game>|install <game> (--archive PATH | --url nxm://…)")
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
		return modsJobCmd("mods.install", "install", []string{"profile", "archive", "url", "name"}, args[1:], func(fs *flag.FlagSet, pos []string) (map[string]any, error) {
			if len(pos) < 1 {
				return nil, errors.New("usage: hikari mods install <game> (--archive PATH | --url nxm://…) [--profile NAME] [--name NAME]")
			}
			params := map[string]any{"game": pos[0]}
			copyFlags(fs, params, "profile", "archive", "url", "name")
			if params["archive"] == nil && params["url"] == nil {
				return nil, errors.New("pass --archive PATH or --url nxm://… to install")
			}
			return params, nil
		})
	}
	return fmt.Errorf("unknown mods command %q", args[0])
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
	if err := client.Subscribe("mods.log", "mods.progress", "mods.done"); err != nil {
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
