import QtQuick
import QtQuick.Controls.Basic
import QtQuick.Layouts
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"

PageBase {
    id: page

    required property var game

    readonly property var info: Daemon.findGame(game.store, game.id)
    readonly property string runState: Daemon.stateOf(game.store, game.id)
    readonly property bool running: info ? Daemon.isRunning(game.store, game.id) : false
    readonly property bool launching: runState === "launching"
    readonly property bool crashed: runState === "crashed"
    readonly property bool installed: info ? info.installed === true : false
    readonly property bool hasUpdate: info ? info.update_available === true : false
    readonly property bool plainWine: !page.settings || !page.settings.wine_version ||
                                      page.settings.wine_version.type !== "proton"
    readonly property bool epicGame: info ? info.store === "epic" : false
    readonly property string statusLine: page.launching ? I18n.tr("Starting the game")
        : (page.running ? I18n.tr("Session active")
        : (page.crashed ? I18n.tr("The last session crashed")
        : (page.installed ? I18n.tr("Ready when you are") : I18n.tr("Choose your install options"))))
    readonly property string key: Daemon.gameKey(game.store, game.id)
    readonly property int detailPanelWidth: content.width > Tokens.px(640)
        ? Math.floor((content.width - Tokens.s4) / 2) : content.width

    property var installInfo: null
    property bool installInfoTried: false
    property var launchPlan: null
    property var settings: null
    property var draft: null
    property string actionNote: ""
    property var wineVersions: []
    property bool winePickerOpen: false
    property var launchOptions: []
    property int launchOptionIndex: -1
    property bool repairing: false
    property bool settingsSheetOpen: false
    property bool commandSheetOpen: false
    property bool logSheetOpen: false
    property var lastLog: null

    readonly property bool sheetOpen: settingsSheetOpen || commandSheetOpen || logSheetOpen

    property var opti: ({ status: null, gpu: null, latestTag: "" })
    property bool optiAdvanced: false
    property string optiInjection: "dxgi.dll"
    property string optiUpscaler: "auto"
    property bool optiFrameGen: false
    property string optiPhase: ""

    Keys.onPressed: event => {
        var step = Math.max(80, scroller.height * 0.6);
        switch (event.key) {
        case Qt.Key_PageDown:
            scroller.contentY = Math.min(scroller.contentHeight - scroller.height,
                                         scroller.contentY + step);
            event.accepted = true;
            break;
        case Qt.Key_PageUp:
            scroller.contentY = Math.max(0, scroller.contentY - step);
            event.accepted = true;
            break;
        case Qt.Key_Home:
            scroller.contentY = 0;
            event.accepted = true;
            break;
        case Qt.Key_End:
            scroller.contentY = scroller.contentHeight;
            event.accepted = true;
            break;
        }
    }

    Component.onCompleted: page.load()
    onKeyChanged: page.load()
    Connections {
        target: Daemon
        function onWineProgress(ev) {
            if (ev.done && !ev.error) {
                page.refreshWineVersions();
            }
        }
        function onGameState(ev) {
            if (ev.store !== game.store || ev.id !== game.id) return;
            if (ev.state !== "crashed") return;
            if (ev.error) page.actionNote = ev.error;
            page.loadLastLog(function (rec) {
                var quick = false;
                if (rec && rec.started_at && rec.ended_at) {
                    var began = Date.parse(rec.started_at), ended = Date.parse(rec.ended_at);
                    quick = isFinite(began) && isFinite(ended) && (ended - began) < 15000;
                }
                if (quick) page.logSheetOpen = true;
            });
        }
    }
    onInfoChanged: page.loadInstallInfo()
    onOptiAdvancedChanged: if (page.optiAdvanced) page.loadOptiReleases()

    function load() {
        page.installInfo = null;
        page.installInfoTried = false;
        page.launchPlan = null;
        page.settings = null;
        page.draft = null;
        Daemon.call("game.settings.get", { store: game.store, id: game.id },
                    (r, e) => {
                        page.settings = e ? null : r;
                        page.draft = e ? null : Object.assign({}, r);
                        settingsSheet.syncFields();
                    });
        page.optiRefresh();
        Daemon.call("opti.detect", {}, (r, e) => {
            if (!e && r && r.gpus && r.gpus.length > 0)
                page.opti = Object.assign({}, page.opti, { gpu: r.gpus[0] });
        });
        page.loadInstallInfo();
        page.refreshWineVersions();
        page.loadLaunchOptions();
    }

    function refreshWineVersions() {
        Daemon.call("wine.versions", null, (r, e) => {
            if (!e && r) page.wineVersions = r.versions || [];
        });
    }

    function loadLaunchOptions() {
        page.launchOptions = [];
        page.launchOptionIndex = -1;
        if (!page.info || !page.info.installed) return;
        Daemon.call("game.launch_options", { store: game.store, id: game.id }, (r, e) => {
            if (!e && r && r.options && r.options.length > 0) page.launchOptions = r.options;
        });
    }

    function chooseWine(v) {
        page.setDraft("wine_version", { bin: v.bin, name: v.name, type: v.type });
        page.winePickerOpen = false;
        page.saveNow(I18n.tr("Compatibility layer saved"));
    }

    function toggleExtra(key, value) {
        if (!page.settings || !page.draft) {
            page.actionNote = I18n.tr("Settings are still loading");
            return;
        }
        page.setDraft(key, value);
        page.saveNow(I18n.tr("Saved"));
    }

    function saveNow(note) {
        if (!page.draft || !page.settings) return;
        var next = settingsSheet.takeDraft();
        Daemon.call("game.settings.set", { store: game.store, id: game.id, settings: next }, (r, e) => {
            if (e) { page.actionNote = e.message; return; }
            if (!r) return;
            page.settings = r;
            page.draft = Object.assign({}, r);
            if (note) page.actionNote = note;
        });
    }

    function downloadLatestProton() {
        Daemon.call("wine.releases", null, (r, e) => {
            if (e || !r || !r.releases || r.releases.length === 0) {
                page.actionNote = e ? e.message : I18n.tr("no releases available");
                return;
            }
            var tag = r.releases[0].tag;
            Daemon.call("wine.download", { tag: tag }, (r2, e2) => {
                page.actionNote = e2 ? e2.message : (I18n.tr("Downloading") + " " + tag);
            });
        });
    }

    function repairGame() {
        if (page.repairing) return;
        page.repairing = true;
        page.actionNote = I18n.tr("Verifying the install");
        Daemon.call("game.repair", { store: game.store, id: game.id }, (r, e) => {
            if (e) {
                page.repairing = false;
                page.actionNote = e.message;
            }
        });
    }

    function syncSaves(direction) {
        Daemon.call("game.saves.sync", {
            store: game.store, id: game.id, direction: direction
        }, (r, e) => {
            page.actionNote = e ? e.message
                : (direction === "upload" ? I18n.tr("Saves uploaded") : I18n.tr("Saves downloaded"));
        });
    }

    function removeLocal() {
        Daemon.call("game.remove_local", { id: game.id }, (r, e) => {
            if (e) { page.actionNote = e.message; return; }
            app.closeDetail();
            Daemon.refreshLibrary();
        });
    }

    function toggleFavorite() {
        Daemon.call("game.favorite", { store: game.store, id: game.id }, (r, e) => {
            if (e) { page.actionNote = e.message; return; }
            if (!r) return;
            if (page.info)
                page.info = Object.assign({}, page.info, { favorite: r.favorite === true });
            page.actionNote = r.favorite ? I18n.tr("Pinned") : I18n.tr("Unpinned");
            Daemon.refreshLibrary();
        });
    }

    function loadLastLog(cb) {
        Daemon.call("game.log", { store: game.store, id: game.id }, (r, e) => {
            page.lastLog = e ? null : r;
            if (cb) cb(e ? null : r);
        });
    }

    function openLogSheet() {
        page.logSheetOpen = true;
        page.loadLastLog();
    }

    function loadInstallInfo() {
        if (!page.info || page.info.installed) {
            page.installInfo = null;
            return;
        }
        if (page.installInfoTried) return;
        page.installInfoTried = true;
        Daemon.call("game.install_info", { store: game.store, id: game.id },
                    (r, e) => { page.installInfo = e ? null : r; });
    }

    function loadOptiReleases() {
        if (page.opti.latestTag) return;
        Daemon.call("opti.releases", {}, (r, e) => {
            if (!e && r && r.releases && r.releases.length > 0)
                page.opti = Object.assign({}, page.opti, { latestTag: r.releases[0].tag });
        });
    }

    function optiRefresh() {
        Daemon.call("opti.status", { store: game.store, id: game.id },
                    (r, e) => { page.opti = Object.assign({}, page.opti, { status: e ? null : r }); });
    }
    function optiInstall(opts) {
        var params = Object.assign({ store: game.store, id: game.id }, opts);
        Daemon.call("opti.install", params, (r, e) => {
            page.actionNote = e ? e.message : I18n.tr("OptiScaler installing");
        });
    }
    function optiRemove() {
        Daemon.call("opti.uninstall", { store: game.store, id: game.id }, (r, e) => {
            page.actionNote = e ? e.message : "";
            page.optiRefresh();
        });
    }

    Connections {
        target: Daemon
        function onOptiProgress(ev) {
            if (ev.store !== page.game.store || ev.id !== page.game.id) return;
            if (ev.done) {
                page.optiPhase = "";
                page.optiRefresh();
            } else {
                var pct = (ev.percent !== undefined) ? (" " + ev.percent.toFixed(0) + "%") : "";
                page.optiPhase = ev.phase + pct;
            }
        }
    }

    function launch() {
        var params = { store: game.store, id: game.id };
        if (page.launchOptionIndex >= 0 && page.launchOptions[page.launchOptionIndex])
            params.option = page.launchOptions[page.launchOptionIndex];
        Daemon.call("game.launch", params, (r, e) => {
            page.actionNote = e ? e.message : I18n.tr("Launching");
        });
    }
    function updateGame() {
        Daemon.call("game.update", { store: game.store, id: game.id }, (r, e) => {
            page.actionNote = e ? e.message : I18n.tr("Updating");
        });
    }
    function stop() {
        Daemon.call("game.stop", { store: game.store, id: game.id }, (r, e) => {
            page.actionNote = e ? e.message : I18n.tr("Stopped");
        });
    }
    function install() {
        var p = installPanel.installParams();
        Daemon.call("game.install", {
            store: game.store, id: game.id,
            path: p.path, platform: p.platform, dlcs: p.dlcs
        }, (r, e) => {
            page.actionNote = e ? e.message : I18n.tr("Installing");
        });
    }
    function openCommandSheet() {
        page.commandSheetOpen = true;
        page.launchPlan = null;
        Daemon.call("game.launch", { store: game.store, id: game.id, dry_run: true },
                    (r, e) => { page.launchPlan = (e || !r) ? { error: e ? e.message : "no plan" } : r.plan; });
    }
    function openSettingsSheet() {
        page.settingsSheetOpen = true;
        page.refreshWineVersions();
        settingsSheet.syncFields();
    }

    function fact(value) { return value ? String(value) : "—"; }
    function setDraft(key, value) {
        var next = Object.assign({}, page.draft || {});
        next[key] = value;
        page.draft = next;
    }
    function fmtSize(bytes) {
        if (!bytes) return "—";
        var gib = bytes / (1024 * 1024 * 1024);
        if (gib >= 1) return gib.toFixed(1) + " GiB";
        return Math.round(bytes / (1024 * 1024)) + " MiB";
    }

    function ensureVisible(item) {
        if (!item || !scroller) return;
        var p = item.mapToItem(scroller.contentItem, 0, 0);
        var top = p.y;
        var bottom = top + item.height;
        var margin = Tokens.s3;
        if (top < scroller.contentY + margin) {
            scroller.contentY = Math.max(0, top - margin);
        } else if (bottom > scroller.contentY + scroller.height - margin) {
            var maxY = Math.max(0, scroller.contentHeight - scroller.height);
            scroller.contentY = Math.min(maxY, bottom + margin - scroller.height);
        }
    }
    Connections {
        target: page.Window.window
        function onActiveFocusItemChanged() {
            if (page.Window.window) page.ensureVisible(page.Window.window.activeFocusItem);
        }
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        Rectangle {
            Layout.fillWidth: true
            implicitHeight: Tokens.px(64)
            color: Tokens.paper
            border.width: 0
            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: Tokens.s5
                anchors.rightMargin: Tokens.s5
                spacing: Tokens.s3
                Btn {
                    compact: true
                    text: "← " + I18n.tr("Library")
                    stealFocus: true
                    onAct: app.closeDetail()
                }
                Text {
                    text: I18n.tr("LIBRARY") + "  /  " + (page.info ? page.info.store.toUpperCase() : "")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.letterSpacing: Tokens.trackLabel
                }
                Item { Layout.fillWidth: true }
                Badge {
                    visible: page.hasUpdate
                    text: I18n.tr("UPDATE AVAILABLE")
                    accent: true
                }
            }
            Rectangle {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                height: 1
                color: Tokens.lineSoft
            }
        }

        Flickable {
            id: scroller
            Layout.fillWidth: true
            Layout.fillHeight: true
            contentWidth: width
            contentHeight: content.implicitHeight + Tokens.s5 * 2
            clip: true
            interactive: !page.sheetOpen
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollBar {}

            ColumnLayout {
                id: content
                x: Tokens.s5
                y: Tokens.s5
                width: Math.max(0, scroller.width - Tokens.s5 * 2)
                spacing: Tokens.s5

                GameHero { page: page }

                RowLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s3
                    ActionButton {
                        primary: !page.running
                        armed: !page.launching
                        glyph: page.launching ? "…" : (page.running ? "■" : (page.installed ? "▶" : "↓"))
                        text: page.launching ? I18n.tr("Launching…")
                            : (page.running ? I18n.tr("Stop playing")
                            : (page.installed ? I18n.tr("Play now") : I18n.tr("Install game")))
                        onAct: page.running ? page.stop() : (page.installed ? page.launch() : page.install())
                    }
                    ActionButton {
                        visible: page.installed && page.hasUpdate
                        compact: true
                        glyph: "↑"
                        text: I18n.tr("Update")
                        onAct: page.updateGame()
                    }
                    ActionButton {
                        visible: page.installed
                        compact: true
                        text: I18n.tr("Launch command")
                        onAct: page.openCommandSheet()
                    }
                    ActionButton {
                        compact: true
                        glyph: "⚙"
                        text: I18n.tr("Settings")
                        onAct: page.openSettingsSheet()
                    }
                    ActionButton {
                        visible: page.installed
                        compact: true
                        glyph: "≡"
                        text: I18n.tr("Log")
                        onAct: page.openLogSheet()
                    }
                    ActionButton {
                        compact: true
                        glyph: page.info && page.info.favorite ? "★" : "☆"
                        text: page.info && page.info.favorite ? I18n.tr("Pinned") : I18n.tr("Pin")
                        onAct: page.toggleFavorite()
                    }
                    Item { Layout.fillWidth: true }
                    ColumnLayout {
                        visible: page.actionNote.length > 0 || page.installed
                        spacing: Tokens.s1
                        Layout.alignment: Qt.AlignRight
                        Text {
                            text: page.statusLine
                            color: page.crashed ? Tokens.alert : Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            horizontalAlignment: Text.AlignRight
                        }
                        Text {
                            visible: page.actionNote.length > 0
                            text: page.actionNote
                            color: Tokens.sunDeep
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            horizontalAlignment: Text.AlignRight
                        }
                    }
                }

                GameInstallPanel { id: installPanel; page: page }

                GameFactsPanel { page: page }

                Entrance {
                    index: 3
                    Layout.fillWidth: true
                    OptiPanel { page: page }
                }
                Item { Layout.preferredHeight: Tokens.s2 }
            }
        }
    }

    GameSettingsSheet { id: settingsSheet; page: page }
    LaunchCommandSheet { page: page }
    LaunchLogSheet { page: page }
}
