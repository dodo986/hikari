package library

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	store string
	games []Game
	err   error
}

func (f fakeProvider) Store() string { return f.store }

func (f fakeProvider) List(ctx context.Context) ([]Game, error) { return f.games, f.err }

func TestServiceListAggregatesAndSorts(t *testing.T) {
	svc := NewService(
		fakeProvider{store: StoreGOG, games: []Game{{Store: StoreGOG, ID: "2", Title: "Beta", Runner: RunnerGogdl, Source: SourceHikari}}},
		fakeProvider{store: StoreEpic, games: []Game{{Store: StoreEpic, ID: "1", Title: "Alpha", Runner: RunnerLegendary, Source: SourceHikari}}},
	)
	games, err := svc.List(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 2 || games[0].Title != "Alpha" || games[1].Title != "Beta" {
		t.Fatalf("bad aggregation/sort: %+v", games)
	}
}

func TestServiceListFilters(t *testing.T) {
	svc := NewService(fakeProvider{store: StoreEpic, games: []Game{
		{Store: StoreEpic, ID: "1", Title: "A", Installed: true},
		{Store: StoreEpic, ID: "2", Title: "B"},
	}})
	games, err := svc.List(context.Background(), StoreEpic, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 || games[0].ID != "1" {
		t.Fatalf("installed filter failed: %+v", games)
	}
	if games, _ := svc.List(context.Background(), StoreGOG, false); len(games) != 0 {
		t.Fatalf("store filter failed: %+v", games)
	}
}

func TestServiceBrokenProviderDoesNotHideOthers(t *testing.T) {
	svc := NewService(
		fakeProvider{store: StoreGOG, err: errors.New("engine missing")},
		fakeProvider{store: StoreEpic, games: []Game{{Store: StoreEpic, ID: "1", Title: "A"}}},
	)
	games, err := svc.List(context.Background(), "", false)
	if err != nil {
		t.Fatalf("one broken provider should not fail the list: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("want 1 game from healthy provider, got %+v", games)
	}

	svc = NewService(fakeProvider{store: StoreGOG, err: errors.New("engine missing")})
	if _, err := svc.List(context.Background(), "", false); err == nil {
		t.Fatal("all-broken list should report the error")
	}
}

func TestServiceGet(t *testing.T) {
	svc := NewService(fakeProvider{store: StoreEpic, games: []Game{{Store: StoreEpic, ID: "1", Title: "A"}}})
	if _, err := svc.Get(context.Background(), "nope", "1"); err == nil {
		t.Fatal("unknown store should error")
	}
	if _, err := svc.Get(context.Background(), StoreEpic, "404"); err == nil {
		t.Fatal("unknown game should error")
	}
	g, err := svc.Get(context.Background(), StoreEpic, "1")
	if err != nil || g.Title != "A" {
		t.Fatalf("get failed: %v %+v", err, g)
	}
}
