package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"hikari/internal/api"
	"hikari/internal/art"
	"hikari/internal/artwork"
	"hikari/internal/config"
	"hikari/internal/dxvk"
	"hikari/internal/engine"
	"hikari/internal/launch"
	"hikari/internal/library"
	"hikari/internal/logging"
	"hikari/internal/mods"
	"hikari/internal/opti"
	"hikari/internal/providers/amazon"
	"hikari/internal/providers/demo"
	"hikari/internal/providers/epic"
	"hikari/internal/providers/gog"
	"hikari/internal/providers/heroic"
	"hikari/internal/providers/local"
	"hikari/internal/providers/steam"
	"hikari/internal/runtimes"
	umuresolver "hikari/internal/umu"
	"hikari/internal/watcher"
	"hikari/internal/winemgr"
)

type daemon struct {
	log      *slog.Logger
	paths    config.Paths
	settings config.Settings
	state    *config.StateStore
	library  *library.Service
	eng      *engine.Manager
	run      *engine.Runner
	games    *launch.Manager
	umu      *umuresolver.Resolver
	art      *art.Cache
	artwork  *artwork.Searcher
	wine     *winemgr.Manager
	started  time.Time

	logPath string
	logRing *logging.Ring
	version string

	fetchMu      sync.Mutex
	fetchPending map[string]bool

	demo     map[string]*demo.Provider
	local    *local.Provider
	opti     opti.Manager
	mods     mods.Service
	tools    *dxvk.Installer
	runtimes *runtimes.Installer

	watchInterval time.Duration

	jobsMu sync.Mutex
	jobs   map[string]*installJob

	modsMu         sync.Mutex
	modsJob        string
	modsCancel     context.CancelFunc
	modsSettingsAt time.Time
	modsSetupTried bool
}

type installJob struct {
	id     string
	cancel context.CancelFunc
}

func cacheDir() string {
	if c := os.Getenv("XDG_CACHE_HOME"); c != "" {
		return c
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(home, ".cache")
}

func newDaemon(log *slog.Logger, demoMode bool) (*daemon, error) {
	paths := config.Resolve()
	if override := os.Getenv("HIKARI_HEROIC_CONFIG"); override != "" {
		paths.HeroicConfig = override
	}
	if err := paths.Ensure(); err != nil {
		return nil, fmt.Errorf("cannot create hikari directories: %w", err)
	}
	settings, err := config.LoadSettings(paths)
	if err != nil {
		return nil, err
	}
	if settings.Library.HeroicConfig != "" {
		paths.HeroicConfig = settings.Library.HeroicConfig
	}
	state, err := config.LoadState(paths)
	if err != nil {
		_ = os.Rename(paths.State, paths.State+".corrupt")
		log.Error("state.json was corrupt; moved aside and started fresh", "file", paths.State+".corrupt", "err", err)
		state, err = config.LoadState(paths)
		if err != nil {
			return nil, err
		}
	}

	runner := engine.NewRunner(log)
	eng := engine.NewManager(settings.Engines, runner)

	localGames := local.New(filepath.Join(paths.DataDir, "local_games.json"))
	var providers []library.Provider
	var demoProviders map[string]*demo.Provider
	if demoMode {
		demoProviders = map[string]*demo.Provider{}
		demoProviders = demo.New(log)
		for _, store := range []string{
			library.StoreEpic, library.StoreGOG, library.StoreAmazon, library.StoreSteam,
		} {
			providers = append(providers, demoProviders[store])
		}
		log.Info("demo mode: fake providers, no engines", "games", 12)
	} else {
		if settings.Library.ImportHeroic {
			providers = append(providers, heroic.New(paths.HeroicConfig))
			log.Info("heroic import enabled", "root", paths.HeroicConfig)
		}
		gogProvider := gog.New(eng, runner, paths)
		gogProvider.RunInPrefix = func(ctx context.Context, settings config.GameSettings, argv []string, installDir string) error {
			return launch.RunInPrefix(ctx, runner, settings, eng, argv, installDir)
		}
		providers = append(providers,
			epic.New(eng, runner, paths),
			gogProvider,
			amazon.New(eng, runner, paths),
			steam.New(),
		)
	}
	providers = append(providers, localGames)

	optiMgr := opti.Manager(opti.NewLocalManager(
		filepath.Join(paths.DataDir, "optiscaler", "versions"), nil))
	if demoMode {
		optiMgr = opti.NewFake()
	}

	var modsService mods.Service = mods.NewFake()
	if !demoMode {
		modsService = mods.New(mods.Options{
			Root:      filepath.Join(paths.DataDir, "mods"),
			Python:    settings.Mods.Python,
			Filegraph: settings.Mods.Filegraph,
			Log:       log,
			Runner:    runner,
		})
	}

	return &daemon{
		log:      log,
		paths:    paths,
		settings: settings,
		state:    state,
		library:  library.NewService(providers...),
		eng:      eng,
		run:      runner,
		games:    launch.NewManager(log, runner),
		umu:      umuresolver.NewResolver(filepath.Join(paths.DataDir, "umu_ids.json")),
		art:      art.New(filepath.Join(cacheDir(), "hikari", "art")),
		artwork:  artwork.New(settings.Library.SteamGridDBKey),
		wine:     winemgr.New(paths.DataDir),
		started:  time.Now(),

		fetchPending: map[string]bool{},
		demo:         demoProviders,
		opti:         optiMgr,
		mods:         modsService,
		local:        localGames,
		tools:        dxvk.NewInstaller(filepath.Join(paths.DataDir, "tools"), log),
		runtimes:     runtimes.NewInstaller(filepath.Join(paths.DataDir, "runtimes"), log),
		jobs:         map[string]*installJob{},
	}, nil
}

func (d *daemon) register(srv *api.Server, ctx context.Context) {
	srv.Register("daemon.status", d.handleStatus)
	srv.Register("daemon.stop", d.handleStop)
	d.registerLibrary(srv)
	d.registerAuth(srv)
	d.registerGame(srv)
	d.registerLaunch(srv)
	d.registerLogs(srv)
	d.registerUpdate(srv)
	d.registerLocal(srv)
	d.registerArtwork(srv)
	d.registerOpti(srv)
	d.registerMods(srv)
	srv.Register("engine.versions", func(c context.Context, params json.RawMessage) (any, error) {
		return map[string]any{"engines": d.eng.Status(c)}, nil
	})
	srv.Register("library.refresh", d.handleRefresh)
	d.registerWine(srv)
	d.watchStateFiles(ctx, srv)
	if len(d.demo) > 0 {
		if err := d.demo[library.StoreEpic].SeedArtwork(d.art); err != nil {
			d.log.Warn("demo artwork seeding failed", "err", err)
		}
	}
}

func (d *daemon) watchStateFiles(ctx context.Context, srv *api.Server) {
	interval := d.watchInterval
	if interval == 0 {
		interval = 3 * time.Second
	}
	paths := map[string]struct {
		store string
		auth  bool
	}{
		filepath.Join(d.paths.EngineEpic, "installed.json"):                                   {"epic", false},
		filepath.Join(d.paths.EngineEpic, "metadata"):                                         {"epic", false},
		filepath.Join(d.paths.EngineEpic, "user.json"):                                        {"epic", true},
		filepath.Join(d.paths.DataDir, "libraries", "gog.json"):                               {"gog", false},
		filepath.Join(d.paths.EngineGOG, "heroic_gogdl", "manifests"):                         {"gog", false},
		filepath.Join(d.paths.EngineGOG, "auth.json"):                                         {"gog", true},
		filepath.Join(d.paths.EngineAmazon, "library.json"):                                   {"amazon", false},
		filepath.Join(d.paths.EngineAmazon, "installed.json"):                                 {"amazon", false},
		filepath.Join(d.paths.EngineAmazon, "current_user.json"):                              {"amazon", true},
		filepath.Join(d.paths.HeroicConfig, "store_cache"):                                    {"heroic", false},
		filepath.Join(d.paths.HeroicConfig, "legendaryConfig", "legendary", "installed.json"): {"heroic", false},
		filepath.Join(d.paths.HeroicConfig, "gog_store", "installed.json"):                    {"heroic", false},
		filepath.Join(d.paths.HeroicConfig, "nile_config", "nile", "installed.json"):          {"heroic", false},
		filepath.Join(d.paths.HeroicConfig, "GamesConfig"):                                    {"heroic", false},
	}
	keys := make([]string, 0, len(paths))
	for k := range paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	w := watcher.New(interval, func(changed []string) {
		stores := map[string]bool{}
		authStores := map[string]bool{}
		for _, p := range changed {
			if meta, ok := paths[p]; ok {
				if meta.auth {
					authStores[meta.store] = true
				} else {
					stores[meta.store] = true
				}
			}
		}
		for store := range stores {
			srv.Publish("library.changed", map[string]any{"store": store})
		}
		for store := range authStores {
			srv.Publish("auth.changed", map[string]any{"store": store, "logged_in": d.storeLoggedIn(ctx, store)})
		}
	})
	w.Add(keys...)
	w.Snapshot()
	go w.Run(ctx)
}

func (d *daemon) handleStatus(c context.Context, params json.RawMessage) (any, error) {
	stores := make([]map[string]any, 0, len(d.library.Providers()))
	for _, p := range d.library.Providers() {
		entry := map[string]any{"store": p.Store(), "available": true}
		if auth, ok := p.(library.Authenticator); ok {
			st, err := auth.AuthStatus(c)
			if err == nil {
				entry["logged_in"] = st.LoggedIn
				if st.User != "" {
					entry["user"] = st.User
				}
			}
		}
		stores = append(stores, entry)
	}
	return map[string]any{
		"version":        version,
		"uptime_seconds": int(time.Since(d.started).Seconds()),
		"socket":         config.SocketPath(),
		"stores":         stores,
		"engines":        d.eng.Status(c),
	}, nil
}

func (d *daemon) handleStop(c context.Context, params json.RawMessage) (any, error) {
	go func() {
		<-time.After(100 * time.Millisecond)
		d.log.Info("shutdown requested via IPC")
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	}()
	return map[string]any{}, nil
}

func (d *daemon) handleRefresh(c context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Store string `json:"store"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store?}")
		}
	}
	refreshed := map[string]int{}
	failed := map[string]string{}
	for _, prov := range d.library.Providers() {
		if p.Store != "" && prov.Store() != p.Store {
			continue
		}
		r, ok := prov.(library.Refresher)
		if !ok {
			continue
		}
		n, err := r.Refresh(c)
		if err != nil {

			failed[prov.Store()] = err.Error()
			continue
		}
		refreshed[prov.Store()] = n
	}
	if p.Store != "" {
		if msg, ok := failed[p.Store]; ok {
			return nil, api.Errorf(api.CodeNotAvailable, "%s refresh failed: %s", p.Store, msg)
		}
	}
	res := map[string]any{"refreshed": refreshed}
	if len(failed) > 0 {
		res["errors"] = failed
	}
	return res, nil
}

func (d *daemon) markInstalled(store, id string, removed bool) {
	_ = d.state.UpdateEntry(store, id, func(e *config.GameEntry) {
		e.Imported = false
	})
}

func (d *daemon) beginJob(key string) bool {
	d.jobsMu.Lock()
	defer d.jobsMu.Unlock()
	if _, busy := d.jobs[key]; busy {
		return false
	}
	d.jobs[key] = &installJob{id: fmt.Sprintf("%d", time.Now().UnixNano())}
	return true
}

func (d *daemon) ownJob(key string, job *installJob) bool {
	d.jobsMu.Lock()
	defer d.jobsMu.Unlock()
	if cur, busy := d.jobs[key]; busy && cur.cancel == nil {
		d.jobs[key] = job
		return true
	}
	return false
}

func (d *daemon) endJob(key string) {
	d.jobsMu.Lock()
	defer d.jobsMu.Unlock()
	delete(d.jobs, key)
}

func (d *daemon) settingsFor(g library.Game) config.GameSettings {
	var imported *config.GameSettings
	if g.Source == library.SourceHeroicImport {
		if hp, ok := d.library.Provider(library.StoreHeroic).(library.SettingsSource); ok {
			if s, found := hp.DefaultSettings(g); found {
				imported = &s
			}
		}
	}
	return launch.EffectiveSettings(g, d.settings.Defaults, d.state.Entry(g.Store, g.ID), imported)
}
