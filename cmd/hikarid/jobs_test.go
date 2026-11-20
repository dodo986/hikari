package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"hikari/internal/api"
)

// Regression: game.uninstall claims the key with a job that has no cancel
// function (beginJob only stores the id; ownJob installs the cancel later, and
// the uninstall path never installs one because it runs synchronously). A
// cancel request arriving in that window dereferenced the nil cancel and
// panicked the daemon; a panic on the unix transport kills the process.
func TestCancelInstallWithPendingJobDoesNotPanic(t *testing.T) {
	d, client := startDaemon(t)

	key := "epic/nativegame"
	if !d.beginJob(key) {
		t.Fatal("the key was already busy")
	}
	defer d.endJob(key)

	var out map[string]any
	err := client.Call("game.install.cancel", map[string]any{"store": "epic", "id": "nativegame"}, &out)
	if err == nil {
		t.Fatal("canceling a job that has no cancel function must report an error, not report success")
	}
	var rpcErr *api.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("cancel error = %v, want a JSON-RPC error", err)
	}
	if rpcErr.Code == api.CodeInternal {
		t.Fatalf("cancel of an uncancelable job is a client error, got internal: %s", rpcErr.Message)
	}

	// The daemon must still answer after the cancel request.
	if err := client.Call("daemon.status", nil, &out); err != nil {
		t.Fatalf("the daemon stopped answering: %v", err)
	}
}

// A cancel that does arrive while a cancelable job owns the key must still
// reach that job's context.
func TestCancelInstallCancelsTheClaimedJob(t *testing.T) {
	d, client := startDaemon(t)

	key := "epic/nativegame"
	if !d.beginJob(key) {
		t.Fatal("the key was already busy")
	}
	defer d.endJob(key)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	job := "install-test"
	if !d.ownJob(key, &installJob{id: job, cancel: cancel}) {
		t.Fatal("could not take over the claimed job")
	}

	var out map[string]any
	if err := client.Call("game.install.cancel", map[string]any{"store": "epic", "id": "nativegame"}, &out); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("the job context was not canceled")
	}
}
