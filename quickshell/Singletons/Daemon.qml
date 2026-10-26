pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

Singleton {
    id: rpc

    readonly property string socketPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/hikari.sock"

    property bool connected: false
    property var games: []
    property var runningGames: []
    property var gameStates: ({})
    property var authStores: ({})
    property var engines: ({})
    property var daemonStatus: null
    property var updateInfo: null
    property bool updateApplying: false
    property string updatePhase: ""
    property real updatePercent: -1
    property var installs: ({})
    property var modsStatus: null
    property var modsGames: []
    property var modsProgress: null
    property var modsLog: []
    property bool modsBusy: false
    property var modsInstalled: null
    property var modsBrowse: null
    property var modsFiles: null
    property var modsAccount: null
    property string modsAuthLink: ""
    property string modsError: ""
    property bool modsSearching: false
    property bool modsFilesBusy: false
    property var modsWait: null

    signal gameState(var ev)
    signal updateProgress(var ev)
    signal installProgress(var ev)
    signal libraryChanged(var ev)
    signal authChanged(var ev)
    signal optiProgress(var ev)
    signal wineProgress(var ev)
    signal modsChanged(var ev)
    signal modsAuthURL(string url)
    signal modsJobGame(string game)

    function gameKey(store, id) { return store + "/" + id; }

    function findGame(store, id) {
        for (var i = 0; i < rpc.games.length; i++)
            if (rpc.games[i].store === store && rpc.games[i].id === id)
                return rpc.games[i];
        return null;
    }

    function isRunning(store, id) {
        var key = rpc.gameKey(store, id);
        for (var i = 0; i < rpc.runningGames.length; i++)
            if (rpc.gameKey(rpc.runningGames[i].store, rpc.runningGames[i].id) === key)
                return true;
        return false;
    }

    function stateOf(store, id) {
        var key = rpc.gameKey(store, id);
        for (var i = 0; i < rpc.runningGames.length; i++)
            if (rpc.gameKey(rpc.runningGames[i].store, rpc.runningGames[i].id) === key)
                return rpc.runningGames[i].state || "playing";
        return rpc.gameStates[key] || "";
    }

    property int _nextId: 1
    property var _pending: ({})

    function call(method, params, cb) {
        var id = rpc._nextId++;
        var req = { jsonrpc: "2.0", id: id, method: method };
        if (params !== undefined && params !== null)
            req.params = params;
        rpc._pending[id] = cb || null;
        var line = JSON.stringify(req) + "\n";
        if (ctl.connected) {
            ctl.write(line);
            ctl.flush();
        } else {
            ctl.queued += line;
        }
        return id;
    }

    function refreshLibrary() { call("library.list", {}, r => applyGames(r)); }
    function refreshRunning() { call("game.running", {}, r => rpc.runningGames = (r && r.games) || []); }
    function refreshAuth() { call("auth.status", {}, r => rpc.authStores = (r && r.stores) || {}); }
    function refreshEngines() { call("engine.versions", {}, r => rpc.engines = (r && r.engines) || {}); }
    function refreshStatus() { call("daemon.status", null, r => rpc.daemonStatus = r); }
    function refreshMods() {
        call("mods.status", null, r => rpc.modsStatus = r);
        call("mods.games", null, r => rpc.modsGames = (r && r.games) || []);
    }
    function refreshModsAccount() {
        call("mods.account", null, r => rpc.modsAccount = r);
    }
    function loadModsInstalled(game, profile) {
        if (!game) return;
        rpc.modsInstalled = null;
        call("mods.installed", { game: game, profile: profile || "" },
             r => rpc.modsInstalled = r);
    }
    function browseMods(game, query, sort, page) {
        if (!game) return;
        rpc.modsSearching = true;
        call("mods.browse", { game: game, query: query || "", sort: sort || "", page: page || 0 },
             (r, e) => {
            rpc.modsSearching = false;
            rpc.modsBrowse = e ? { game: game, results: [], error: e.message } : r;
        });
    }
    function loadModFiles(game, modId) {
        if (!game || !modId) return;
        rpc.modsFilesBusy = true;
        rpc.modsFiles = null;
        call("mods.files", { game: game, mod_id: modId }, (r, e) => {
            rpc.modsFilesBusy = false;
            rpc.modsFiles = e ? { error: e.message } : r;
        });
    }
    function installNexusMod(game, profile, modId, fileId) {
        rpc.modsFiles = null;
        modsJob("mods.install", { game: game, profile: profile || "", mod_id: modId, file_id: fileId });
    }
    function loginNexus() {
        rpc.modsAuthLink = "";
        modsJob("mods.login", {});
    }
    function logoutNexus() {
        call("mods.logout", null, () => {
            rpc.modsAccount = { stored: false };
            rpc.modsBrowse = null;
            rpc.modsChanged({ ok: true, action: "nexus" });
        });
    }
    function setNexusKey(key) {
        call("mods.nexus", { action: "set", key: key }, r => {
            rpc.modsChanged({ ok: true, action: "nexus" });
            rpc.refreshModsAccount();
        });
    }

    property string _jobGameSeen: ""

    // Announce the game once per job: a link from the browser should bring the
    // window to that game's mods page instead of leaving the user to find it.
    function noteJobGame(ev) {
        if (!ev || !ev.job || ev.job === rpc._jobGameSeen) return;
        rpc._jobGameSeen = ev.job;
        if (ev.game) rpc.modsJobGame(ev.game);
    }

    function modsJob(method, params) {
        if (modsBusy) return;
        modsBusy = true;
        modsLog = [];
        modsProgress = null;
        modsError = "";
        call(method, params, (r, e) => {
            if (e) {
                modsBusy = false;
                modsError = e.message;
                modsChanged({ ok: false, error: e.message });
            }
        });
    }
    function deployMods(game, profile) { modsJob("mods.deploy", { game: game, profile: profile || "" }); }
    function restoreMods(game) { modsJob("mods.restore", { game: game }); }
    function installMod(game, profile, archive) {
        modsJob("mods.install", { game: game, profile: profile || "", archive: archive });
    }
    function installFromFile(game, profile, archive, name) {
        modsJob("mods.install", { game: game, profile: profile || "",
                                  archive: archive, name: name || "" });
    }
    // A free account installs by downloading the archive in the browser: watch
    // the download folder until it lands, then hand it to the normal install.
    function waitForDownload(game, profile, modId, fileId, name) {
        rpc.modsWait = { game: game, profile: profile || "", mod_id: modId,
                         file_id: fileId, name: name || "", ticks: 0 };
        rpc.modsError = "";
        waitPoller.start();
        pollDownload();
    }
    function stopWaiting() {
        waitPoller.stop();
        rpc.modsWait = null;
    }
    function pollDownload() {
        var w = rpc.modsWait;
        if (!w) return;
        w.ticks++;
        if (w.ticks > 300) {
            rpc.stopWaiting();
            rpc.modsError = "Nothing arrived from the browser in 15 minutes; the Nexus page is still open if you want to try again.";
            return;
        }
        rpc.call("mods.downloads", { game: w.game, mod_id: w.mod_id, file_id: w.file_id }, (r, e) => {
            if (e || !r || !r.found) return;
            // The archive keeps sitting there; install it once the slot is free.
            if (rpc.modsBusy) return;
            var path = String(r.found);
            var name = w.name;
            rpc.stopWaiting();
            rpc.installFromFile(w.game, w.profile, path, name);
        });
    }
    function setupMods() { modsJob("mods.setup", {}); }
    function cancelMods() { call("mods.cancel", null, () => {}); }
    function updateText() {
        if (updateApplying) {
            var pct = updatePercent >= 0 ? "  " + Math.round(updatePercent) + "%" : "";
            return (updatePhase || I18n.tr("Updating")) + pct;
        }
        if (updatePhase === "failed") return I18n.tr("Update failed — retry");
        return I18n.tr("Update available") + ": " + (updateInfo ? updateInfo.latest : "");
    }
    function checkUpdates() {
        if (!connected || updateApplying) return;
        call("app.update.check", null, (r, e) => {
            if (e) return;
            rpc.updateInfo = (r && r.available) ? r : null;
            if (rpc.updateInfo) rpc.updatePhase = "";
        });
    }
    function applyUpdate() {
        if (updateApplying) return;
        updateApplying = true;
        updatePhase = I18n.tr("Preparing");
        updatePercent = -1;
        call("app.update.apply", null, (r, e) => {
            if (e) {
                updateApplying = false;
                updatePhase = "failed";
                updatePercent = -1;
            }
        });
    }
    function refreshAll() {
        refreshLibrary(); refreshRunning(); refreshAuth(); refreshEngines(); refreshStatus();
    }

    function applyGames(res) {
        if (res && res.games)
            rpc.games = res.games;
    }

    Socket {
        id: sub
        path: rpc.socketPath
        parser: SplitParser { onRead: line => rpc.applyEvent(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                var req = { jsonrpc: "2.0", id: -1, method: "events.subscribe", params: {} };
                write(JSON.stringify(req) + "\n");
                flush();
                rpc.connected = true;
                rpc.checkUpdates();
                rpc.refreshAll();
            } else {
                
                rpc.connected = false;
                sub.connected = false;
                ctl.connected = false;
            }
        }
    }

    Socket {
        id: ctl
        path: rpc.socketPath
        property string queued: ""
        parser: SplitParser { onRead: line => rpc.applyReply(line) }
        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                flushQueued();
            } else {
                ctl.connected = false;
                rpc.connected = false;
            }
        }
    }

    Timer {
        id: waitPoller
        interval: 3000
        repeat: true
        onTriggered: rpc.pollDownload()
    }

    Timer {
        id: retry
        interval: 2000
        repeat: true
        running: true
        onTriggered: {
            if (!sub.connected) sub.connected = true;
            if (!ctl.connected) ctl.connected = true;
        }
    }

    function applyEvent(line) {
        var msg;
        try { msg = JSON.parse(line); } catch (e) { return; }
        if (!msg || !msg.method)
            return;
        var ev = msg.params || {};
        switch (msg.method) {
        case "install.progress":
            var key = rpc.gameKey(ev.store, ev.id);
            var inst = Object.assign({}, rpc.installs);
            if (ev.state === "done" || ev.state === "error" || ev.state === "canceled") {
                if (ev.state === "done") {
                    delete inst[key];
                } else {
                    inst[key] = ev;
                }
                rpc.installs = inst;
            } else {
                inst[key] = ev;
                rpc.installs = inst;
            }
            installProgress(ev);
            break;
        case "game.state":
            var stateKey = rpc.gameKey(ev.store, ev.id);
            var states = Object.assign({}, rpc.gameStates);
            if (ev.state === "stopped") delete states[stateKey];
            else states[stateKey] = ev.state;
            rpc.gameStates = states;
            gameState(ev);
            refreshRunning();
            refreshLibrarySoon.restart();
            break;
        case "library.changed":
            libraryChanged(ev);
            refreshLibrarySoon.restart();
            break;
        case "auth.changed":
            authChanged(ev);
            refreshAuth();
            break;
        case "opti.progress":
            optiProgress(ev);
            break;
        case "update.progress":
            updateProgress(ev);
            if (ev.phase === "error") {
                rpc.updateApplying = false;
                rpc.updatePhase = "failed";
                rpc.updatePercent = -1;
            } else {
                rpc.updateApplying = ev.phase !== "done";
                rpc.updatePhase = ev.phase;
                rpc.updatePercent = (ev.percent !== undefined && ev.percent !== null) ? ev.percent : -1;
            }
            break;
        case "wine.progress":
            wineProgress(ev);
            break;
        case "mods.progress":
            rpc.noteJobGame(ev);
            rpc.modsProgress = ev;
            break;
        case "mods.log":
            rpc.noteJobGame(ev);
            rpc.modsBusy = true;
            var tail = rpc.modsLog.slice(-7);
            tail.push(ev.line || "");
            rpc.modsLog = tail;
            break;
        case "mods.auth_url":
            rpc.modsAuthLink = ev.url || "";
            rpc.modsAuthURL(ev.url || "");
            Qt.openUrlExternally(ev.url || "");
            break;
        case "mods.done":
            rpc.modsBusy = false;
            rpc.modsProgress = null;
            rpc.modsAuthLink = "";
            // A job that fails after it started answers here, not in call().
            if (!ev.ok && ev.error) rpc.modsError = ev.error;
            rpc.modsChanged(ev);
            if (ev.ok) {
                rpc.refreshMods();
                rpc.refreshModsAccount();
            }
            break;
        }
    }

    Timer {
        id: refreshLibrarySoon
        interval: 250
        onTriggered: rpc.refreshLibrary()
    }

    function applyReply(line) {
        var msg;
        try { msg = JSON.parse(line); } catch (e) { return; }
        if (!msg || msg.id === undefined)
            return;
        var cb = rpc._pending[msg.id];
        delete rpc._pending[msg.id];
        if (cb) {
            if (msg.error)
                cb(null, msg.error);
            else
                cb(msg.result !== undefined ? msg.result : {}, null);
        }
    }

    Process {
        id: starter
        command: ["systemctl", "--user", "start", "hikarid"]
        onExited: (code, status) => {
            if (code !== 0) {
                fallback.running = true;
            }
        }
    }
    Process { id: fallback; command: ["hikarid"] }

    function startDaemon() {
        starter.running = true;
        retry.interval = 600;
        retry.restart();
        retry.interval = 2000;
    }

    function stopDaemon() { call("daemon.stop", null, null); }
}
