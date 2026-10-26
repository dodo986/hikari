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

    signal gameState(var ev)
    signal updateProgress(var ev)
    signal installProgress(var ev)
    signal libraryChanged(var ev)
    signal authChanged(var ev)
    signal optiProgress(var ev)
    signal wineProgress(var ev)
    signal modsChanged(var ev)

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

    function modsJob(method, params) {
        if (modsBusy) return;
        modsBusy = true;
        modsLog = [];
        modsProgress = null;
        call(method, params, (r, e) => {
            if (e) {
                modsBusy = false;
                modsChanged({ ok: false, error: e.message });
            }
        });
    }
    function deployMods(game, profile) { modsJob("mods.deploy", { game: game, profile: profile || "" }); }
    function restoreMods(game) { modsJob("mods.restore", { game: game }); }
    function installMod(game, profile, archive) {
        modsJob("mods.install", { game: game, profile: profile || "", archive: archive });
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
            rpc.modsProgress = ev;
            break;
        case "mods.log":
            var tail = rpc.modsLog.slice(-7);
            tail.push(ev.line || "");
            rpc.modsLog = tail;
            break;
        case "mods.done":
            rpc.modsBusy = false;
            rpc.modsProgress = null;
            rpc.modsChanged(ev);
            if (ev.ok) rpc.refreshMods();
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
