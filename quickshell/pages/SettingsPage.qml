import QtQuick
import QtQuick.Controls.Basic
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"

PageBase {
    id: page

    property var status: null
    readonly property var engines: Daemon.engines
    readonly property var engineOrder: ["legendary", "gogdl", "nile", "comet", "umu", "wine"]
    readonly property var enginePurpose: {
        "legendary": I18n.tr("Epic Games: store, library and installs"),
        "gogdl": I18n.tr("GOG: store, library and installs"),
        "nile": I18n.tr("Amazon Games: store, library and installs"),
        "comet": I18n.tr("GOG achievements and cloud saves in game"),
        "umu": I18n.tr("Runs Windows games through Proton"),
        "wine": I18n.tr("Runs Windows games through Wine")
    }
    readonly property int cardWidth: body.width > Tokens.px(700)
        ? Math.floor((body.width - Tokens.s4) / 2) : body.width

    property var wineVersions: []
    property var wineReleases: []
    property string wineStatus: ""
    property bool wineError: false
    property bool wineBusy: false

    property var logTail: null
    property string reportStatus: ""

    property bool artKeyStored: false
    property string artKeyStatus: ""
    property bool artKeyError: false

    Component.onCompleted: {
        Daemon.refreshStatus();
        refreshWines();
        loadLogs();
    }

    Connections {
        target: Daemon
        function onWineProgress(ev) {
            if (ev.phase === "error") {
                page.wineError = true;
                page.wineStatus = ev.error || I18n.tr("download failed");
                page.wineBusy = false;
                return;
            }
            if (ev.done) {
                page.wineBusy = false;
                page.wineStatus = I18n.tr("Installed") + " " + (ev.name || "");
                page.wineError = false;
                page.refreshWines();
                return;
            }
            page.wineBusy = true;
            var pct = (ev.percent !== undefined && ev.percent !== null)
                ? (" " + Math.round(ev.percent) + "%") : "";
            page.wineStatus = ev.phase + pct + (ev.name ? (" · " + ev.name) : "");
        }
    }

    function refreshWines() {
        Daemon.call("wine.versions", null, (r, e) => {
            if (!e && r) page.wineVersions = r.versions || [];
        });
    }

    function toggleWineReleases() {
        if (page.wineReleases.length > 0) {
            page.wineReleases = [];
            return;
        }
        page.wineStatus = I18n.tr("loading releases…");
        Daemon.call("wine.releases", null, (r, e) => {
            if (e) {
                page.wineError = true;
                page.wineStatus = e.message;
                return;
            }
            page.wineError = false;
            page.wineStatus = "";
            page.wineReleases = r.releases || [];
        });
    }

    function downloadWine(tag) {
        if (page.wineBusy) return;
        Daemon.call("wine.download", { tag: tag }, (r, e) => {
            if (e) {
                page.wineError = true;
                page.wineStatus = e.message;
                return;
            }
            page.wineBusy = true;
            page.wineStatus = I18n.tr("starting");
        });
    }

    function removeWine(name) {
        Daemon.call("wine.remove", { name: name }, (r, e) => {
            page.wineError = !!e;
            page.wineStatus = e ? e.message : (I18n.tr("Removed") + " " + name);
            page.refreshWines();
            Daemon.refreshEngines();
        });
    }
    Connections {
        target: Daemon
        function onDaemonStatusChanged() { page.status = Daemon.daemonStatus; }
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: Tokens.s5
        spacing: Tokens.s4

        RowLayout {
            Layout.fillWidth: true
            ColumnLayout {
                spacing: Tokens.s1
                Text {
                    text: I18n.tr("Settings")
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fTitle
                }
                Text {
                    text: I18n.tr("The daemon and the engines behind the library.")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }
            }
        }

        Flickable {
            Layout.fillWidth: true
            Layout.fillHeight: true
            contentWidth: width
            contentHeight: body.implicitHeight
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollBar {}

            ColumnLayout {
                id: body
                width: parent.width
                spacing: Tokens.s4

                Panel {
                    Layout.fillWidth: true
                    title: I18n.tr("Daemon")
                    subtitle: I18n.tr("One process owns the socket, the library and every engine run.")
                    RowLayout {
                        Layout.fillWidth: true
                        spacing: Tokens.s4
                        ColumnLayout {
                            spacing: Tokens.s1
                            Text {
                                text: page.status ? "hikari " + page.status.version : I18n.tr("hikari")
                                color: Tokens.ink
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fRow
                                font.weight: Font.Medium
                            }
                            Text {
                                text: page.status ? (I18n.tr("up") + " " + page.fmtUptime(page.status.uptime_seconds)) : ""
                                color: Tokens.inkFaint
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                            }
                        }
                        Item { Layout.fillWidth: true }
                        ColumnLayout {
                            spacing: Tokens.s1
                            Layout.alignment: Qt.AlignRight
                            Text {
                                text: I18n.tr("SOCKET")
                                color: Tokens.inkFaint
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fTiny
                                font.letterSpacing: Tokens.trackMark
                                horizontalAlignment: Text.AlignRight
                                Layout.alignment: Qt.AlignRight
                            }
                            Text {
                                text: page.status ? page.status.socket : ""
                                color: Tokens.inkDim
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fSmall
                                elide: Text.ElideMiddle
                                Layout.maximumWidth: Tokens.px(380)
                            }
                        }
                    }
                }

                Panel {
                    Layout.fillWidth: true
                    title: I18n.tr("Wine & Proton")
                    subtitle: I18n.tr("Builds Hikari can run games with, and one-click Proton downloads from the same releases Heroic uses.")

                    Text {
                        text: I18n.tr("INSTALLED")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny
                        font.letterSpacing: Tokens.trackMark
                    }
                    Repeater {
                        model: page.wineVersions
                        RowLayout {
                            required property var modelData
                            Layout.fillWidth: true
                            spacing: Tokens.s3
                            Text {
                                Layout.fillWidth: true
                                text: modelData.name
                                color: Tokens.ink
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                                elide: Text.ElideRight
                            }
                            Badge {
                                text: modelData.type === "proton" ? I18n.tr("PROTON") : I18n.tr("WINE")
                            }
                            Badge {
                                text: modelData.source.toUpperCase()
                                accent: modelData.source === "hikari"
                            }
                            ActionButton {
                                compact: true
                                visible: modelData.source === "hikari"
                                text: I18n.tr("Remove")
                                onAct: page.removeWine(modelData.name)
                            }
                        }
                    }
                    Text {
                        visible: page.wineVersions.length === 0
                        text: I18n.tr("none found")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }

                    RowLayout {
                        spacing: Tokens.s3
                        ActionButton {
                            compact: true
                            text: page.wineReleases.length > 0 ? I18n.tr("Hide downloads") : I18n.tr("Download Proton")
                            onAct: page.toggleWineReleases()
                        }
                        Text {
                            Layout.fillWidth: true
                            text: page.wineStatus
                            color: page.wineError ? Tokens.alert : Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            elide: Text.ElideRight
                        }
                    }

                    Repeater {
                        model: page.wineReleases.slice(0, 8)
                        RowLayout {
                            required property var modelData
                            Layout.fillWidth: true
                            spacing: Tokens.s3
                            Text {
                                Layout.fillWidth: true
                                text: modelData.tag + "  ·  " + modelData.variant
                                color: Tokens.ink
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                                elide: Text.ElideRight
                            }
                            Text {
                                text: Math.round(modelData.size / 1024 / 1024) + " MiB"
                                color: Tokens.inkFaint
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fTiny
                            }
                            ActionButton {
                                compact: true
                                text: I18n.tr("Install")
                                onAct: page.downloadWine(modelData.tag)
                            }
                        }
                    }
                }

                Text {
                    text: I18n.tr("ENGINES")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                    font.letterSpacing: Tokens.trackMark
                }
                Flow {
                    Layout.fillWidth: true
                    spacing: Tokens.s4
                    Repeater {
                        model: page.engineOrder
                        Entrance {
                            id: engineCard
                            required property string modelData
                            width: page.cardWidth
                            readonly property var engine: page.engines[modelData] || ({ available: false })
                            Panel {
                                width: parent.width
                                RowLayout {
                                    Layout.fillWidth: true
                                    spacing: Tokens.s2
                                    Text {
                                        Layout.fillWidth: true
                                        text: engineCard.modelData
                                        color: Tokens.ink
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fRow
                                        font.weight: Font.Medium
                                    }
                                    Badge {
                                        text: engineCard.engine.available ? I18n.tr("READY") : I18n.tr("MISSING")
                                        accent: engineCard.engine.available
                                        danger: !engineCard.engine.available
                                    }
                                }
                                Text {
                                    Layout.fillWidth: true
                                    text: page.enginePurpose[engineCard.modelData]
                                    color: Tokens.inkFaint
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fSmall
                                    wrapMode: Text.WordWrap
                                }
                                Text {
                                    Layout.fillWidth: true
                                    text: engineCard.engine.available
                                        ? page.engineDetail(engineCard.engine)
                                        : I18n.tr("not installed")
                                    color: engineCard.engine.available ? Tokens.inkDim : Tokens.alert
                                    font.family: engineCard.engine.available ? Tokens.mono : Tokens.ui
                                    font.pixelSize: Tokens.fSmall
                                    elide: Text.ElideMiddle
                                }
                            }
                        }
                    }
                }
                Panel {
                    Layout.fillWidth: true
                    title: I18n.tr("Diagnostics")
                    subtitle: I18n.tr("The daemon's own log. Copy a report when something fails; it has versions, engine paths and the recent log, no credentials.")
                    RowLayout {
                        Layout.fillWidth: true
                        spacing: Tokens.s3
                        Text {
                            Layout.fillWidth: true
                            text: page.logTail ? page.logTail.path : ""
                            color: Tokens.inkFaint
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                            elide: Text.ElideMiddle
                        }
                        ActionButton {
                            compact: true
                            text: I18n.tr("Refresh")
                            onAct: { page.loadLogs(); page.reportStatus = ""; }
                        }
                        ActionButton {
                            compact: true
                            text: I18n.tr("Copy report")
                            onAct: page.copyReport()
                        }
                    }
                    Text {
                        Layout.fillWidth: true
                        visible: page.reportStatus.length > 0
                        text: page.reportStatus
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }
                    LogView {
                        text: page.logTail && page.logTail.lines ? page.logTail.lines.join("\n") : ""
                    }
                    TextEdit { id: reportBuffer; visible: false; width: 1; height: 1 }
                }

                Panel {
                    Layout.fillWidth: true
                    title: I18n.tr("Artwork")
                    subtitle: I18n.tr("Hikari fills missing covers from Steam. A SteamGridDB key covers the games Steam does not have, and it stays on this machine.")
                    RowLayout {
                        Layout.fillWidth: true
                        spacing: Tokens.s3
                        Rectangle {
                            Layout.fillWidth: true
                            implicitHeight: Tokens.ctlH
                            radius: Tokens.radius
                            color: Tokens.paper
                            border.width: Tokens.border
                            border.color: keyField.activeFocus ? Tokens.line : Tokens.lineSoft
                            TextField {
                                id: keyField
                                anchors.fill: parent
                                anchors.leftMargin: Tokens.s3
                                anchors.rightMargin: Tokens.s3
                                background: null
                                color: Tokens.ink
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                                placeholderText: page.artKeyStored ? I18n.tr("a key is stored, paste a new one to replace it") : I18n.tr("paste your SteamGridDB key")
                                placeholderTextColor: Tokens.inkFaint
                                echoMode: TextInput.Password
                                selectByMouse: true
                            }
                        }
                        ActionButton {
                            compact: true
                            text: I18n.tr("Save")
                            armed: keyField.text.trim().length > 8
                            onAct: page.saveArtKey(keyField.text.trim())
                        }
                        ActionButton {
                            compact: true
                            visible: page.artKeyStored
                            text: I18n.tr("Forget")
                            onAct: page.saveArtKey("")
                        }
                    }
                    Text {
                        Layout.fillWidth: true
                        visible: page.artKeyStatus.length > 0
                        text: page.artKeyStatus
                        color: page.artKeyError ? Tokens.ink : Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.WordWrap
                    }
                }

                Item { Layout.fillHeight: true }
            }
        }
    }

    function saveArtKey(key) {
        page.artKeyStatus = I18n.tr("checking")
        Daemon.call("artwork.key", { key: key }, (r, e) => {
            if (e) {
                page.artKeyError = true;
                page.artKeyStatus = e.message || I18n.tr("that key was rejected");
                return;
            }
            page.artKeyError = false;
            page.artKeyStored = !!(r && r.stored);
            page.artKeyStatus = page.artKeyStored
                ? I18n.tr("saved, covers are being looked up again")
                : I18n.tr("forgotten, Steam art will be used");
            keyField.text = "";
            Daemon.refreshLibrary();
        });
    }

    function loadLogs() {
        Daemon.call("logs.tail", { lines: 80 }, (r, e) => {
            if (!e && r) page.logTail = r;
        });
    }

    function copyReport() {
        Daemon.call("logs.report", null, (r, e) => {
            if (e) { page.reportStatus = e.message; return; }
            if (!r) { page.reportStatus = I18n.tr("no report available"); return; }
            reportBuffer.text = r.text;
            reportBuffer.selectAll();
            reportBuffer.copy();
            page.reportStatus = I18n.tr("Report copied to the clipboard");
        });
    }

    function engineDetail(engine) {
        if (engine.version && engine.version.toLowerCase().indexOf("usage:") !== 0)
            return engine.version;
        return engine.path || I18n.tr("installed");
    }

    function fmtUptime(sec) {
        if (!sec) return "0s";
        if (sec < 60) return sec + "s";
        if (sec < 3600) return Math.floor(sec / 60) + "m";
        return Math.floor(sec / 3600) + "h " + Math.floor((sec % 3600) / 60) + "m";
    }
}
