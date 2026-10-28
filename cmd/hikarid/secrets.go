package main

import (
	"context"
	"encoding/json"
	"strings"

	"hikari/internal/api"
	"hikari/internal/config"
	"hikari/internal/secrets"
)

// registerSecrets exposes API keys as write only. A client can set one and can
// read whether it exists, never its value.
func (d *daemon) registerSecrets(srv *api.Server) {
	srv.Register("settings.secretStatus", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(params, &p)
		if p.Name != "" && !secrets.Known(p.Name) {
			return nil, api.Errorf(api.CodeInvalidParams, "%q is not a secret this daemon stores", p.Name)
		}
		names := []string{secrets.SteamGridDB}
		if p.Name != "" {
			names = []string{p.Name}
		}
		out := make([]secrets.Status, 0, len(names))
		for _, n := range names {
			out = append(out, d.secrets.Status(n))
		}
		return map[string]any{
			"secrets":    out,
			"encryption": d.secrets.Encrypted(),
		}, nil
	})

	srv.Register("settings.setSecret", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {name, value}")
		}
		if !secrets.Known(p.Name) {
			return nil, api.Errorf(api.CodeInvalidParams, "%q is not a secret this daemon stores", p.Name)
		}
		value := strings.TrimSpace(p.Value)
		if value != "" && p.Name == secrets.SteamGridDB {
			if err := d.artwork.Check(c, value); err != nil {
				return nil, api.Errorf(api.CodeInvalidParams, "%s", err)
			}
		}
		if err := d.secrets.Set(p.Name, value); err != nil {
			return nil, api.Errorf(api.CodeInternal, "%s", err)
		}
		if p.Name == secrets.SteamGridDB {
			d.artwork.SetKey(value)
		}
		return map[string]any{
			"secret":     d.secrets.Status(p.Name),
			"encryption": d.secrets.Encrypted(),
		}, nil
	})
}

// migrateSecrets moves keys that used to live in plaintext settings.toml into
// the keyring and blanks the file field, so an upgrade does not keep a copy.
func migrateSecrets(log interface {
	Warn(msg string, args ...any)
}, store *secrets.Store, paths config.Paths, settings *config.Settings) {
	plain := settings.Library.SteamGridDBKey
	if strings.TrimSpace(plain) == "" {
		return
	}
	replacement, err := store.Migrate(secrets.SteamGridDB, plain)
	if err != nil {
		log.Warn("cannot move the SteamGridDB key into the keyring", "err", err)
		return
	}
	if replacement == plain {
		return
	}
	settings.Library.SteamGridDBKey = replacement
	if err := config.SaveSettings(paths, *settings); err != nil {
		log.Warn("cannot rewrite settings.toml after the key migration", "err", err)
		return
	}
	log.Warn("moved the SteamGridDB key out of settings.toml", "encrypted", store.Encrypted())
}
