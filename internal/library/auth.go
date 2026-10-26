package library

import (
	"context"
	"time"
)

type AuthStatus struct {
	LoggedIn bool   `json:"logged_in"`
	User     string `json:"user,omitempty"`
	UserID   string `json:"user_id,omitempty"`
}

type LoginInfo struct {
	URL  string `json:"url"`
	Hint string `json:"hint"`
}

type Authenticator interface {
	AuthStatus(ctx context.Context) (AuthStatus, error)
	BeginLogin(ctx context.Context) (LoginInfo, error)
	CompleteLogin(ctx context.Context, code string) (AuthStatus, error)
	Logout(ctx context.Context) error
}

type Refresher interface {
	Refresh(ctx context.Context) (int, error)
}

type DemoLauncher interface {
	DemoLaunchLife(g Game) time.Duration
}
