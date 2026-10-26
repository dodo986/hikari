package main

import (
	"context"
	"sync"
	"time"

	"encoding/json"
	"hikari/internal/api"
	"hikari/internal/browserlogin"
	"hikari/internal/library"
)

func (d *daemon) registerAuth(srv *api.Server) {
	authenticatorFor := func(store string) (library.Authenticator, error) {
		p := d.library.Provider(store)
		if p == nil {
			return nil, api.Errorf(api.CodeInvalidParams, "unknown store: %s", store)
		}
		a, ok := p.(library.Authenticator)
		if !ok {
			return nil, api.Errorf(api.CodeUnsupported, "store %s has no login", store)
		}
		return a, nil
	}

	srv.Register("auth.status", func(c context.Context, params json.RawMessage) (any, error) {
		stores := map[string]library.AuthStatus{}
		for _, p := range d.library.Providers() {
			a, ok := p.(library.Authenticator)
			if !ok {
				continue
			}
			st, err := a.AuthStatus(c)
			if err != nil {
				st = library.AuthStatus{}
			}
			stores[p.Store()] = st
		}
		return map[string]any{"stores": stores}, nil
	})

	srv.Register("auth.login", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store string `json:"store"`
			Code  string `json:"code"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, code?}")
		}
		a, err := authenticatorFor(p.Store)
		if err != nil {
			return nil, err
		}
		if p.Code == "" {
			info, err := a.BeginLogin(c)
			if err != nil {
				return nil, err
			}
			return map[string]any{"state": "pending", "url": info.URL, "hint": info.Hint}, nil
		}
		st, err := a.CompleteLogin(c, p.Code)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		srv.Publish("auth.changed", map[string]any{"store": p.Store, "logged_in": st.LoggedIn})
		return map[string]any{"state": "ok", "user": map[string]any{"name": st.User, "id": st.UserID}}, nil
	})

	var autoMu sync.Mutex
	autoBusy := map[string]bool{}
	srv.Register("auth.login.auto", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store string `json:"store"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store}")
		}
		if d.demo != nil {
			return nil, api.Errorf(api.CodeUnsupported, "demo mode has no real store login")
		}
		a, err := authenticatorFor(p.Store)
		if err != nil {
			return nil, err
		}
		autoMu.Lock()
		if autoBusy[p.Store] {
			autoMu.Unlock()
			return nil, api.Errorf(api.CodeBusy, "a %s login is already open", p.Store)
		}
		autoBusy[p.Store] = true
		autoMu.Unlock()
		defer func() {
			autoMu.Lock()
			delete(autoBusy, p.Store)
			autoMu.Unlock()
		}()

		info, err := a.BeginLogin(c)
		if err != nil {
			return nil, err
		}
		loginURL := browserlogin.LoginURL(p.Store, info.URL)
		if loginURL == "" {
			return nil, api.Errorf(api.CodeNotAvailable, "%s returned no login URL", p.Store)
		}
		sess, err := browserlogin.Open(c, p.Store, loginURL)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		defer sess.Close()

		waitCtx, cancel := context.WithTimeout(c, 5*time.Minute)
		defer cancel()
		code, err := sess.WaitCode(waitCtx, p.Store)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "automatic %s login: %s", p.Store, err)
		}
		st, err := a.CompleteLogin(c, code)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		srv.Publish("auth.changed", map[string]any{"store": p.Store, "logged_in": st.LoggedIn})
		return map[string]any{"state": "ok", "user": map[string]any{"name": st.User, "id": st.UserID}}, nil
	})

	srv.Register("auth.logout", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store string `json:"store"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store}")
		}
		a, err := authenticatorFor(p.Store)
		if err != nil {
			return nil, err
		}
		if err := a.Logout(c); err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		srv.Publish("auth.changed", map[string]any{"store": p.Store, "logged_in": false})
		return map[string]any{}, nil
	})
}

func (d *daemon) storeLoggedIn(ctx context.Context, store string) bool {
	if a, ok := d.library.Provider(store).(library.Authenticator); ok {
		st, err := a.AuthStatus(ctx)
		return err == nil && st.LoggedIn
	}
	return false
}
