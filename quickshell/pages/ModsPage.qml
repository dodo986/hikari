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

    readonly property var status: Daemon.modsStatus
    readonly property bool ready: page.status !== null && page.status.available === true
    readonly property var games: Daemon.modsGames
    readonly property var readyGames: page.games.filter(g => g.configured)
    readonly property var restGames: page.games.filter(g => !g.configured)
    readonly property var game: page.selectedGame()

    property var chosen: ({})
    property bool showAll: false
    property string tab: "installed"
    property string query: ""
    property string sort: "downloads"
    property string pendingGame: ""
    property var installingFile: null
    property bool showArchived: false

    readonly property var sortKeys: ["created", "updated", "downloads", "endorsements"]
    readonly property var sortLabels: ({
        "created": I18n.tr("New"),
        "updated": I18n.tr("Updated"),
        "downloads": I18n.tr("Popular"),
        "endorsements": I18n.tr("Endorsed")
    })
    // Cards are a fixed size, like the library's tiles: whole columns, no stretching.
    readonly property int cardW: Tokens.px(224)
    readonly property int cardH: Math.round(cardW * 0.56) + Tokens.px(176)
    readonly property int columns: Math.max(1, Math.floor((browsePane.width + Tokens.s4) / (cardW + Tokens.s4)))

    function selectedGame() {
        var id = app.modGame;
        if (!id) return null;
        for (var i = 0; i < page.games.length; i++)
            if (page.games[i].id === id) return page.games[i];
        return null;
    }
    function artFor(entry) {
        if (!entry) return null;
        var known = Daemon.games.find(g => g.store === entry.store && g.id === entry.game_id);
        return { title: entry.name, artwork_local: known ? known.artwork_local : ({}) };
    }
    function profileFor(entry) {
        if (!entry) return "";
        if (page.chosen[entry.id]) return page.chosen[entry.id];
        if (entry.deployed_profile) return entry.deployed_profile;
        return entry.profiles && entry.profiles.length > 0 ? entry.profiles[0] : "";
    }
    function cycleProfile(entry) {
        var list = entry.profiles || [];
        if (list.length < 2) return;
        var next = list[(list.indexOf(page.profileFor(entry)) + 1) % list.length];
        var copy = Object.assign({}, page.chosen);
        copy[entry.id] = next;
        page.chosen = copy;
        Daemon.loadModsInstalled(entry.id, next);
    }
    function pickSort(key) {
        if (page.sort === key) return;
        page.sort = key;
        page.search(0);
    }
    function shortDate(value) {
        if (!value) return "";
        var date = new Date(value);
        if (isNaN(date.getTime())) return "";
        return Qt.formatDateTime(date, "d MMM yyyy");
    }
    function search(pageIndex) {
        if (!Daemon.modsAccount || !Daemon.modsAccount.stored) return;
        if (!page.game) return;
        Daemon.browseMods(page.game.id, page.query, page.sort, pageIndex || 0);
    }
    function openGame(entry) {
        app.openModGame(entry.id);
        page.tab = "installed";
        Daemon.loadModsInstalled(entry.id, page.profileFor(entry));
    }
    function applyTab() {
        if (!app.modTab) return;
        page.tab = app.modTab;
        app.modTab = "";
        if (page.tab === "browse" && !Daemon.modsBrowse) page.search(0);
    }
    function installArchive(entry) {
        page.pendingGame = entry.id;
        archivePicker.open();
    }
    function usableFiles() {
        if (!Daemon.modsFiles || !Daemon.modsFiles.files) return [];
        return Daemon.modsFiles.files.filter(function (file) {
            var retired = file.category === "ARCHIVED" || file.category === "REMOVED";
            return page.showArchived || !retired;
        });
    }
    function hiddenFiles() {
        if (!Daemon.modsFiles || !Daemon.modsFiles.files) return 0;
        return Daemon.modsFiles.files.length - page.usableFiles().length;
    }
    // Premium accounts get the file straight from the API. Free accounts press
    // "Mod manager download" on the Nexus page; the archive then lands in the
    // download folder and gets installed without another click.
    function directDownloads() {
        return !Daemon.modsAccount || Daemon.modsAccount.premium !== false;
    }
    function nexusFileURL(mod, fileId) {
        if (!page.game || !page.game.nexus_domain || !mod) return "";
        return "https://www.nexusmods.com/" + page.game.nexus_domain + "/mods/" + mod.mod_id
               + "?tab=files&file_id=" + fileId;
    }
    function installFile(file) {
        if (page.directDownloads()) {
            Daemon.installNexusMod(page.game.id, page.profileFor(page.game),
                                   page.installingFile.mod_id, file.file_id);
            return;
        }
        Qt.openUrlExternally(page.nexusFileURL(page.installingFile, file.file_id));
        Daemon.waitForDownload(page.game.id, page.profileFor(page.game),
                               page.installingFile.mod_id, file.file_id, file.name);
    }
    function pickFile(mod) {
        page.installingFile = mod;
        page.showArchived = false;
        Daemon.loadModFiles(page.game.id, mod.mod_id);
        filesSheet.shown = true;
    }
    function compact(value) {
        var n = Number(value || 0);
        if (n >= 1000000) return (n / 1000000).toFixed(n >= 10000000 ? 0 : 1) + "M";
        if (n >= 1000) return (n / 1000).toFixed(n >= 10000 ? 0 : 1) + "K";
        return String(n);
    }

    Connections {
        target: Daemon
        function onConnectedChanged() {
            if (Daemon.connected) Daemon.refreshMods();
        }
        function onModsChanged(ev) {
            if (!ev || !ev.ok) return;
            Daemon.refreshMods();
            if (page.game) Daemon.loadModsInstalled(page.game.id, page.profileFor(page.game));
        }
    }

    Component.onCompleted: {
        Daemon.refreshMods();
        Daemon.refreshModsAccount();
    }
    onTabChanged: {
        if (page.tab === "browse" && Daemon.modsBrowse === null) page.search(0);
    }
    Connections {
        target: Daemon
        // A finished login makes browsing possible; load what the tab is waiting for.
        function onModsAccountChanged() {
            if (page.tab === "browse" && page.game && Daemon.modsBrowse === null) page.search(0);
        }
    }
    onGamesChanged: {
        // A deep link can arrive before the game list does; search once it is here.
        if (page.tab === "browse" && page.game && Daemon.modsBrowse === null) page.search(0);
    }
    onGameChanged: {
        if (page.game) {
            page.tab = "installed";
            Daemon.loadModsInstalled(page.game.id, page.profileFor(page.game));
            if (Daemon.modsBrowse && Daemon.modsBrowse.game !== page.game.name) Daemon.modsBrowse = null;
            Qt.callLater(page.applyTab);
        }
    }

    Keys.onEscapePressed: event => {
        if (filesSheet.shown) {
            filesSheet.shown = false;
            event.accepted = true;
            return;
        }
        if (keySheet.shown) {
            keySheet.shown = false;
            event.accepted = true;
            return;
        }
        if (app.modGame) {
            app.closeModGame();
            event.accepted = true;
        }
    }

    // ---- games list -------------------------------------------------------
    ColumnLayout {
        anchors.fill: parent
        anchors.margins: Tokens.s5
        spacing: Tokens.s4
        visible: !page.game

        RowLayout {
            Layout.fillWidth: true
            spacing: Tokens.s3
            ColumnLayout {
                spacing: Tokens.s1
                Text {
                    text: I18n.tr("Mods")
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fTitle
                }
                Text {
                    text: page.readyGames.length + " " + I18n.tr("ready to mod") + "  ·  " +
                          page.restGames.length + " " + I18n.tr("without a game folder")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }
            }
            Item { Layout.fillWidth: true }
            Badge {
                visible: page.status !== null
                text: page.ready ? I18n.tr("MODS READY") : I18n.tr("BACKEND OFFLINE")
                accent: page.ready
                danger: page.status !== null && !page.ready
            }
            RefreshButton {
                armed: !Daemon.modsBusy
                onAct: Daemon.refreshMods()
            }
        }

        Panel {
            visible: page.status !== null && !page.ready
            Layout.fillWidth: true
            title: I18n.tr("Modding backend unavailable")
            subtitle: page.status ? page.status.reason : ""
            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.s3
                Text {
                    Layout.fillWidth: true
                    text: I18n.tr("The modding backend ships inside Hikari and needs an interpreter with its Python packages. Set it up here, once.")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.WordWrap
                }
                Btn {
                    compact: true
                    text: I18n.tr("Set up")
                    primary: true
                    armed: !Daemon.modsBusy
                    onAct: Daemon.setupMods()
                }
            }
        }

        JobPanel { visible: Daemon.modsBusy; Layout.fillWidth: true }

        Panel {
            visible: Daemon.modsError !== ""
            Layout.fillWidth: true
            title: I18n.tr("That did not work")
            subtitle: Daemon.modsError
            RowLayout {
                Layout.fillWidth: true
                Item { Layout.fillWidth: true }
                Btn {
                    compact: true
                    text: I18n.tr("Dismiss")
                    onAct: Daemon.modsError = ""
                }
            }
        }

        Item {
            Layout.fillWidth: true
            Layout.fillHeight: true

            Flickable {
                id: scroller
                anchors.fill: parent
                contentWidth: width
                contentHeight: body.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollBar {}

                ColumnLayout {
                    id: body
                    width: scroller.width
                    spacing: Tokens.s3

                    Repeater {
                        model: page.readyGames
                        Entrance {
                            required property var modelData
                            Layout.fillWidth: true
                            Rectangle {
                                width: parent.width
                                implicitHeight: Tokens.px(84)
                                radius: Tokens.radius
                                color: hover.containsMouse ? Tokens.paperLift : Tokens.paper
                                border.width: Tokens.border
                                border.color: hover.containsMouse ? Tokens.line : Tokens.lineSoft
                                Behavior on color { ColorAnimation { duration: Tokens.durSmall } }

                                RowLayout {
                                    anchors.fill: parent
                                    anchors.margins: Tokens.s3
                                    spacing: Tokens.s4

                                    GameArt {
                                        Layout.preferredWidth: Tokens.px(60)
                                        Layout.preferredHeight: Tokens.px(60)
                                        game: page.artFor(modelData)
                                        role: "square"
                                        showTitle: false
                                    }
                                    ColumnLayout {
                                        spacing: Tokens.s1
                                        Text {
                                            text: modelData.name
                                            color: Tokens.ink
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fRow
                                            font.weight: Font.Medium
                                        }
                                        Text {
                                            text: page.profileFor(modelData) + "  ·  " +
                                                  (modelData.nexus_domain || I18n.tr("no Nexus domain")) +
                                                  (modelData.store ? "  ·  " + modelData.store : "")
                                            color: Tokens.inkFaint
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                        }
                                    }
                                    Item { Layout.fillWidth: true }
                                    Badge {
                                        visible: modelData.deploy_active
                                        text: I18n.tr("DEPLOYED")
                                        accent: true
                                    }
                                    Text {
                                        text: "›"
                                        color: Tokens.inkFaint
                                        font.family: Tokens.display
                                        font.pixelSize: Tokens.fHero
                                    }
                                }
                                MouseArea {
                                    id: hover
                                    anchors.fill: parent
                                    hoverEnabled: true
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: page.openGame(modelData)
                                }
                            }
                        }
                    }

                    RowLayout {
                        visible: page.restGames.length > 0
                        Layout.fillWidth: true
                        Layout.topMargin: Tokens.s2
                        spacing: Tokens.s3
                        Btn {
                            compact: true
                            text: page.showAll ? I18n.tr("Hide them")
                                               : I18n.tr("Show all") + " " + page.restGames.length
                            onAct: page.showAll = !page.showAll
                        }
                        Text {
                            Layout.fillWidth: true
                            text: I18n.tr("Hikari knows these games too; point one at its folder and it becomes moddable here.")
                            color: Tokens.inkFaint
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                    }

                    Repeater {
                        model: page.showAll ? page.restGames : []
                        Rectangle {
                            required property var modelData
                            Layout.fillWidth: true
                            implicitHeight: Tokens.px(34)
                            radius: Tokens.radius
                            color: Tokens.tint5
                            border.width: Tokens.border
                            border.color: Tokens.lineSoft
                            RowLayout {
                                anchors.fill: parent
                                anchors.leftMargin: Tokens.s4
                                anchors.rightMargin: Tokens.s4
                                spacing: Tokens.s3
                                Text {
                                    text: modelData.name
                                    color: Tokens.inkDim
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fSmall
                                }
                                Item { Layout.fillWidth: true }
                                Btn {
                                    compact: true
                                    text: I18n.tr("Set folder")
                                    onAct: {
                                        page.pendingGame = modelData.id;
                                        gameFolderPicker.open();
                                    }
                                }
                            }
                        }
                    }
                }
            }

            Empty {
                anchors.centerIn: parent
                visible: page.games.length === 0
                caption: page.ready ? I18n.tr("The backend knows no games yet.")
                                    : I18n.tr("The modding backend is not available.")
            }
        }
    }

    // ---- one game ---------------------------------------------------------
    ColumnLayout {
        anchors.fill: parent
        spacing: 0
        visible: page.game !== null

        Rectangle {
            Layout.fillWidth: true
            implicitHeight: Tokens.px(60)
            color: Tokens.paper
            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: Tokens.s5
                anchors.rightMargin: Tokens.s5
                spacing: Tokens.s3
                Btn {
                    compact: true
                    text: "← " + I18n.tr("Mods")
                    stealFocus: true
                    onAct: app.closeModGame()
                }
                Text {
                    text: "MODS  /  " + (page.game ? page.game.name.toUpperCase() : "")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.letterSpacing: Tokens.trackLabel
                    elide: Text.ElideRight
                    Layout.maximumWidth: Tokens.px(320)
                }
                Item { Layout.fillWidth: true }
                Badge {
                    text: page.profileFor(page.game) || I18n.tr("NO PROFILE")
                    muted: !page.game || !page.game.deploy_active
                }
                Badge {
                    visible: page.game && page.game.deploy_active
                    text: I18n.tr("DEPLOYED")
                    accent: true
                }
                Btn {
                    compact: true
                    text: I18n.tr("Install archive")
                    armed: !Daemon.modsBusy && page.game !== null
                    onAct: page.installArchive(page.game)
                }
                Btn {
                    compact: true
                    visible: page.game && page.game.deploy_active
                    text: I18n.tr("Restore")
                    armed: !Daemon.modsBusy
                    onAct: Daemon.restoreMods(page.game.id)
                }
                Btn {
                    compact: true
                    text: page.game && page.game.deploy_active ? I18n.tr("Deploy again") : I18n.tr("Deploy")
                    primary: page.game && !page.game.deploy_active
                    armed: !Daemon.modsBusy && page.game !== null
                    onAct: Daemon.deployMods(page.game.id, page.profileFor(page.game))
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

        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.margins: Tokens.s5
            spacing: Tokens.s4

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.s2
                Btn {
                    text: I18n.tr("Installed") + "  " + ((Daemon.modsInstalled && Daemon.modsInstalled.mods)
                                                         ? Daemon.modsInstalled.mods.length : 0)
                    primary: page.tab === "installed"
                    onAct: page.tab = "installed"
                }
                Btn {
                    text: I18n.tr("Browse Nexus")
                    primary: page.tab === "browse"
                    onAct: {
                        page.tab = "browse";
                        if (!Daemon.modsBrowse) page.search(0);
                    }
                }
                Item { Layout.fillWidth: true }
                Text {
                    visible: Daemon.modsAccount !== null && Daemon.modsAccount.stored
                    text: (Daemon.modsAccount && Daemon.modsAccount.user
                           ? I18n.tr("signed in as") + " " + Daemon.modsAccount.user
                           : I18n.tr("signed in"))
                          + (Daemon.modsAccount && Daemon.modsAccount.premium ? " · " + I18n.tr("premium") : "")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                }
                Btn {
                    compact: true
                    visible: Daemon.modsAccount !== null && Daemon.modsAccount.stored
                    text: I18n.tr("Sign out")
                    onAct: Daemon.logoutNexus()
                }
                Text {
                    text: page.game && page.game.nexus_domain
                          ? "nexusmods.com/" + page.game.nexus_domain
                          : I18n.tr("no Nexus domain for this game")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                }
            }

            JobPanel {
                visible: Daemon.modsBusy
                Layout.fillWidth: true
                actionText: Daemon.modsAuthLink !== "" ? I18n.tr("Open the Nexus page") : ""
                onAction: Qt.openUrlExternally(Daemon.modsAuthLink)
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.s3
                visible: Daemon.modsWait !== null

                Rectangle {
                    Layout.fillWidth: true
                    implicitHeight: waitText.implicitHeight + Tokens.s3 * 2
                    radius: Tokens.radius
                    color: Tokens.tint5
                    border.width: Tokens.border
                    border.color: Tokens.lineSoft
                    Text {
                        id: waitText
                        anchors.fill: parent
                        anchors.margins: Tokens.s3
                        text: I18n.tr("Nexus needs Premium for a direct download, so the file's page is open in your browser. Press “Mod manager download” (or the slow download) — Hikari installs it as soon as it lands.")
                              + "  " + (Daemon.modsWait && Daemon.modsWait.name ? Daemon.modsWait.name : "")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.WordWrap
                    }
                }
                Btn {
                    compact: true
                    text: I18n.tr("Cancel")
                    onAct: Daemon.stopWaiting()
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.s3
                visible: Daemon.modsError !== ""

                Rectangle {
                    Layout.fillWidth: true
                    implicitHeight: alertText.implicitHeight + Tokens.s3 * 2
                    radius: Tokens.radius
                    color: Tokens.tint5
                    border.width: Tokens.border
                    border.color: Tokens.alert
                    Text {
                        id: alertText
                        anchors.fill: parent
                        anchors.margins: Tokens.s3
                        text: Daemon.modsError
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.WordWrap
                        verticalAlignment: Text.AlignVCenter
                    }
                }
                Btn {
                    compact: true
                    text: I18n.tr("Dismiss")
                    onAct: Daemon.modsError = ""
                }
            }

            // Installed -----------------------------------------------------
            ColumnLayout {
                Layout.fillWidth: true
                Layout.fillHeight: true
                spacing: Tokens.s3
                visible: page.tab === "installed"

                RowLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s3
                    Text {
                        text: (Daemon.modsInstalled && Daemon.modsInstalled.profile)
                              ? Daemon.modsInstalled.profile : page.profileFor(page.game)
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fRow
                        font.weight: Font.Medium
                    }
                    Btn {
                        compact: true
                        visible: page.game && (page.game.profiles || []).length > 1
                        text: I18n.tr("Change profile")
                        onAct: page.cycleProfile(page.game)
                    }
                    Item { Layout.fillWidth: true }
                    Text {
                        text: I18n.tr("Click a row to enable or disable it; deploy applies the list.")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny
                    }
                    Btn {
                        compact: true
                        text: I18n.tr("Repair mods")
                        armed: !Daemon.modsHealthBusy && !Daemon.modsBusy
                        onAct: Daemon.repairMods(page.game.id, page.profileFor(page.game))
                    }
                    Btn {
                        compact: true
                        text: I18n.tr("Check requirements")
                        armed: !Daemon.modsMissingBusy
                        onAct: Daemon.checkRequirements(page.game.id, page.profileFor(page.game))
                    }
                    RefreshButton {
                        armed: !Daemon.modsBusy
                        onAct: Daemon.loadModsInstalled(page.game.id, page.profileFor(page.game))
                    }
                }

                // What the health check found, newest run only. A Column, not a
                // ColumnLayout: an empty layout here swallows the list's height.
                Column {
                    id: healthBlock
                    Layout.fillWidth: true
                    spacing: Tokens.s1
                    visible: Daemon.modsHealthBusy || Daemon.modsHealth !== null

                    property var problems: (Daemon.modsHealth || []).filter(function (f) {
                        return f.severity !== "info";
                    })

                    Text {
                        width: parent.width
                        text: Daemon.modsHealthBusy
                              ? I18n.tr("Checking that every mod loader is really in the game folder…")
                              : ""
                        visible: text !== ""
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }

                    Repeater {
                        model: healthBlock.problems
                        Text {
                            required property var modelData
                            width: parent.width
                            text: modelData.detail + (modelData.mod ? "  ·  " + modelData.mod : "")
                            color: modelData.severity === "error" ? Tokens.alert : Tokens.inkFaint
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                    }

                    Text {
                        width: parent.width
                        text: I18n.tr("Nothing missing: every loader file is in the game folder and each loader has a log.")
                        visible: !Daemon.modsHealthBusy && Daemon.modsHealth !== null
                                 && healthBlock.problems.length === 0
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }
                }

                // Nexus requirements the profile does not have yet.
                Column {
                    Layout.fillWidth: true
                    spacing: Tokens.s1
                    visible: Daemon.modsMissingBusy || Daemon.modsMissing !== null

                    Text {
                        width: parent.width
                        text: Daemon.modsMissingBusy
                              ? I18n.tr("Asking Nexus what these mods need…")
                              : (Daemon.modsMissing && Daemon.modsMissing.length === 0
                                 ? I18n.tr("Every listed requirement is installed.")
                                 : "")
                        visible: text !== ""
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }

                    Repeater {
                        model: Daemon.modsMissing || []
                        RowLayout {
                            required property var modelData
                            width: parent.width
                            spacing: Tokens.s3
                            Text {
                                Layout.fillWidth: true
                                text: modelData.mod + " " + I18n.tr("needs")
                                      + " " + (modelData.needs || []).map(function (n) {
                                          return n.name + (n.external ? " (" + I18n.tr("outside Nexus") + ")" : "");
                                      }).join(", ")
                                color: Tokens.alert
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                                elide: Text.ElideRight
                            }
                            Repeater {
                                model: (modelData.needs || []).filter(function (n) { return n.mod_id > 0; })
                                Btn {
                                    required property var modelData
                                    compact: true
                                    text: I18n.tr("Get") + " " + modelData.name
                                    armed: !Daemon.modsBusy
                                    onAct: page.pickFile({ mod_id: modelData.mod_id, name: modelData.name })
                                }
                            }
                        }
                    }
                }

                Flickable {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    contentWidth: width
                    contentHeight: installedBody.implicitHeight
                    clip: true
                    boundsBehavior: Flickable.StopAtBounds
                    ScrollBar.vertical: ScrollBar {}

                    ColumnLayout {
                        id: installedBody
                        width: parent.width
                        spacing: Tokens.s1

                        Repeater {
                            model: Daemon.modsInstalled ? Daemon.modsInstalled.mods : []
                            Rectangle {
                                id: row
                                required property var modelData
                                required property int index
                                Layout.fillWidth: true
                                implicitHeight: Tokens.px(30)
                                radius: Tokens.radius
                                color: rowHover.containsMouse ? Tokens.tint5 : "transparent"
                                property bool confirming: false

                                // Under the row, so the buttons keep their clicks.
                                MouseArea {
                                    id: rowHover
                                    anchors.fill: parent
                                    hoverEnabled: true
                                    onClicked: {
                                        row.confirming = false;
                                        if (Daemon.modsBusy) return;
                                        Daemon.toggleMod(page.game.id, page.profileFor(page.game),
                                                         modelData.name);
                                    }
                                }
                                RowLayout {
                                    anchors.fill: parent
                                    anchors.leftMargin: Tokens.s3
                                    anchors.rightMargin: Tokens.s3
                                    spacing: Tokens.s3
                                    Text {
                                        Layout.preferredWidth: Tokens.px(22)
                                        text: index + 1
                                        horizontalAlignment: Text.AlignRight
                                        color: Tokens.inkFaint
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fTiny
                                    }
                                    Rectangle {
                                        Layout.preferredWidth: Tokens.px(12)
                                        Layout.preferredHeight: Tokens.px(12)
                                        radius: Tokens.px(2)
                                        color: modelData.enabled ? Tokens.sun : "transparent"
                                        border.width: Tokens.border
                                        border.color: modelData.enabled ? Tokens.sun : Tokens.line
                                    }
                                    Text {
                                        text: modelData.name
                                        color: modelData.enabled ? Tokens.ink : Tokens.inkFaint
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fSmall
                                        Layout.fillWidth: true
                                        elide: Text.ElideRight
                                    }
                                    Text {
                                        text: modelData.enabled ? "" : I18n.tr("disabled")
                                        color: Tokens.inkFaint
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fTiny
                                    }
                                    Btn {
                                        compact: true
                                        text: row.confirming ? I18n.tr("Remove?") : I18n.tr("Uninstall")
                                        visible: true
                                        armed: !Daemon.modsBusy
                                        onAct: {
                                            if (!row.confirming) { row.confirming = true; return; }
                                            row.confirming = false;
                                            Daemon.removeMod(page.game.id, page.profileFor(page.game),
                                                             modelData.name);
                                        }
                                    }
                                }
                            }
                        }
                    }
                }

                Empty {
                    anchors.centerIn: parent
                    visible: Daemon.modsInstalled !== null && Daemon.modsInstalled.mods.length === 0
                    caption: I18n.tr("Nothing installed in this profile yet — browse Nexus or install an archive.")
                }
            }

            // Browse --------------------------------------------------------
            ColumnLayout {
                id: browsePane
                Layout.fillWidth: true
                Layout.fillHeight: true
                spacing: Tokens.s3
                visible: page.tab === "browse"

                Panel {
                    visible: Daemon.modsAccount !== null && !Daemon.modsAccount.stored
                    Layout.fillWidth: true
                    title: I18n.tr("Sign in to Nexus")
                    subtitle: I18n.tr("Your browser opens Nexus; approving there is the whole login.")
                    RowLayout {
                        Layout.fillWidth: true
                        spacing: Tokens.s3
                        Item { Layout.fillWidth: true }
                        Btn {
                            compact: true
                            text: I18n.tr("Use an API key")
                            onAct: keySheet.shown = true
                        }
                        Btn {
                            compact: true
                            text: I18n.tr("Sign in")
                            primary: true
                            onAct: Daemon.loginNexus()
                        }
                    }
                }

                RowLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s3
                    visible: Daemon.modsAccount !== null && Daemon.modsAccount.stored

                    Rectangle {
                        Layout.fillWidth: true
                        implicitHeight: Tokens.ctlH
                        radius: Tokens.radius
                        color: searchField.activeFocus ? Tokens.paperLift : Tokens.paper
                        border.width: Tokens.border
                        border.color: searchField.activeFocus ? Tokens.line : Tokens.lineSoft
                        TextField {
                            id: searchField
                            anchors.fill: parent
                            anchors.leftMargin: Tokens.s3
                            anchors.rightMargin: Tokens.s3
                            background: null
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            placeholderText: I18n.tr("Search Nexus, or leave empty for the most downloaded")
                            placeholderTextColor: Tokens.inkFaint
                            selectByMouse: true
                            onTextChanged: page.query = text
                            onAccepted: page.search(0)
                        }
                    }
                    Btn {
                        compact: true
                        text: Daemon.modsSearching ? I18n.tr("Searching…") : I18n.tr("Search")
                        armed: !Daemon.modsSearching
                        onAct: page.search(0)
                    }
                }

                RowLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s2
                    visible: Daemon.modsAccount !== null && Daemon.modsAccount.stored

                    Chips {
                        options: page.sortKeys
                        labels: page.sortLabels
                        current: page.sort
                        onChose: key => page.pickSort(key)
                    }
                    Item { Layout.fillWidth: true }
                    Text {
                        text: Daemon.modsBrowse && Daemon.modsBrowse.results
                              ? Daemon.modsBrowse.results.length + " " + I18n.tr("mods")
                              : ""
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny
                    }
                }

                Flickable {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    contentWidth: width
                    contentHeight: resultsBody.implicitHeight
                    clip: true
                    boundsBehavior: Flickable.StopAtBounds
                    ScrollBar.vertical: ScrollBar {}

                    ColumnLayout {
                        id: resultsBody
                        width: parent.width
                        spacing: Tokens.s2

                        Grid {
                            id: modGrid
                            Layout.fillWidth: true
                            columns: page.columns
                            columnSpacing: Tokens.s4
                            rowSpacing: Tokens.s4

                            Repeater {
                                model: (Daemon.modsBrowse ? Daemon.modsBrowse.results : [])
                                Rectangle {
                                    required property var modelData
                                    width: page.cardW
                                    height: page.cardH
                                    radius: Tokens.radius
                                    color: cardHover.containsMouse ? Tokens.paperLift : Tokens.paper
                                    border.width: Tokens.border
                                    border.color: cardHover.containsMouse ? Tokens.line : Tokens.lineSoft
                                    clip: true

                                    ColumnLayout {
                                        anchors.fill: parent
                                        spacing: 0

                                        Rectangle {
                                            Layout.fillWidth: true
                                            Layout.preferredHeight: Math.round(page.cardW * 0.56)
                                            color: Tokens.tint10
                                            clip: true

                                            Text {
                                                anchors.centerIn: parent
                                                visible: cover.status !== Image.Ready
                                                text: modelData.name.charAt(0).toUpperCase()
                                                color: Tokens.inkFaint
                                                font.family: Tokens.display
                                                font.pixelSize: Math.round(Tokens.fRow * 1.4)
                                            }
                                            Image {
                                                id: cover
                                                anchors.fill: parent
                                                source: modelData.picture_url || ""
                                                // Decode at the drawn size: Nexus serves 1920px artwork.
                                                sourceSize.width: page.cardW * 2
                                                sourceSize.height: page.cardW * 2
                                                asynchronous: true
                                                fillMode: Image.PreserveAspectCrop
                                                cache: true
                                            }
                                            Badge {
                                                anchors.right: parent.right
                                                anchors.top: parent.top
                                                anchors.margins: Tokens.s2
                                                visible: modelData.adult === true
                                                text: I18n.tr("ADULT")
                                                danger: true
                                            }
                                        }

                                        ColumnLayout {
                                            Layout.fillWidth: true
                                            Layout.fillHeight: true
                                            Layout.margins: Tokens.s3
                                            spacing: Tokens.s1

                                            Text {
                                                text: modelData.name
                                                color: Tokens.ink
                                                font.family: Tokens.ui
                                                font.pixelSize: Tokens.fRow
                                                font.weight: Font.Medium
                                                wrapMode: Text.Wrap
                                                maximumLineCount: 2
                                                elide: Text.ElideRight
                                                Layout.fillWidth: true
                                            }
                                            Text {
                                                text: (modelData.author || I18n.tr("unknown author"))
                                                      + (modelData.category ? "  ·  " + modelData.category : "")
                                                color: Tokens.inkFaint
                                                font.family: Tokens.ui
                                                font.pixelSize: Tokens.fTiny
                                                elide: Text.ElideRight
                                                Layout.fillWidth: true
                                            }
                                            Text {
                                                text: modelData.updated
                                                      ? I18n.tr("updated") + " " + page.shortDate(modelData.updated)
                                                      : (modelData.created
                                                         ? I18n.tr("added") + " " + page.shortDate(modelData.created) : "")
                                                color: Tokens.inkFaint
                                                font.family: Tokens.ui
                                                font.pixelSize: Tokens.fTiny
                                                elide: Text.ElideRight
                                                Layout.fillWidth: true
                                            }
                                            Text {
                                                text: modelData.summary || ""
                                                color: Tokens.inkDim
                                                font.family: Tokens.ui
                                                font.pixelSize: Tokens.fSmall
                                                wrapMode: Text.Wrap
                                                maximumLineCount: 3
                                                elide: Text.ElideRight
                                                Layout.fillWidth: true
                                                Layout.fillHeight: true
                                                verticalAlignment: Text.AlignTop
                                            }
                                            Text {
                                                text: (modelData.version ? modelData.version + "  ·  " : "")
                                                      + page.compact(modelData.downloads) + " " + I18n.tr("downloads")
                                                      + "  ·  " + page.compact(modelData.endorsements) + " " + I18n.tr("endorsements")
                                                color: Tokens.inkDim
                                                font.family: Tokens.ui
                                                font.pixelSize: Tokens.fTiny
                                                elide: Text.ElideRight
                                                Layout.fillWidth: true
                                            }
                                        }
                                    }

                                    MouseArea {
                                        id: cardHover
                                        anchors.fill: parent
                                        hoverEnabled: true
                                        cursorShape: Qt.PointingHandCursor
                                        onClicked: page.pickFile(modelData)
                                    }
                                }
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Layout.topMargin: Tokens.s2
                            visible: Daemon.modsBrowse !== null && Daemon.modsBrowse.results
                                     && Daemon.modsBrowse.results.length > 0 && Daemon.modsBrowse.more
                            spacing: Tokens.s3
                            Btn {
                                compact: true
                                text: I18n.tr("Load more")
                                armed: !Daemon.modsSearching
                                onAct: page.search((Daemon.modsBrowse.page || 0) + 1)
                            }
                            Item { Layout.fillWidth: true }
                        }

                        Empty {
                            Layout.fillWidth: true
                            Layout.topMargin: Tokens.s5
                            visible: Daemon.modsAccount !== null && Daemon.modsAccount.stored
                                     && Daemon.modsBrowse !== null
                                     && Daemon.modsBrowse.error !== undefined
                            caption: Daemon.modsBrowse && Daemon.modsBrowse.error
                                     ? Daemon.modsBrowse.error : ""
                        }
                    }
                }
            }
        }
    }

    FileDialogHost {
        id: gameFolderPicker
        title: I18n.tr("Choose the game folder")
        folders: true
        currentFolder: "file://" + (Quickshell.env("HOME") || "/")
        onAccepted: {
            var path = decodeURIComponent(selectedFolder.toString().replace("file://", ""));
            Daemon.call("mods.configure", { "game": page.pendingGame, "path": path }, function () {
                Daemon.refreshMods();
            });
        }
    }

    FileDialogHost {
        id: archivePicker
        title: I18n.tr("Choose a mod archive")
        currentFolder: "file://" + (Quickshell.env("HOME") || "/")
        nameFilters: [I18n.tr("Archives (*.zip *.7z *.rar *.tar.gz *.tar.xz)"), I18n.tr("All files (*)")]
        onAccepted: {
            var path = decodeURIComponent(selectedFile.toString().replace("file://", ""));
            var entry = page.games.find(g => g.id === page.pendingGame);
            if (entry) Daemon.installMod(entry.id, page.profileFor(entry), path);
        }
    }

    Sheet {
        id: filesSheet
        title: page.installingFile ? page.installingFile.name : I18n.tr("Files")
        subtitle: I18n.tr("Pick the file to download and install; it unpacks into this profile.")
        onCloseRequested: filesSheet.shown = false

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: Tokens.s6
            spacing: Tokens.s2

            Repeater {
                model: page.usableFiles()
                Rectangle {
                    required property var modelData
                    Layout.fillWidth: true
                    implicitHeight: Tokens.px(48)
                    radius: Tokens.radius
                    color: Tokens.paper
                    border.width: Tokens.border
                    border.color: Tokens.lineSoft
                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: Tokens.s4
                        anchors.rightMargin: Tokens.s3
                        spacing: Tokens.s3
                        ColumnLayout {
                            spacing: 0
                            Text {
                                text: modelData.name
                                color: Tokens.ink
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                            }
                            Text {
                                text: (modelData.category || "") + "  ·  " + (modelData.version || "") +
                                      "  ·  " + (modelData.size || "") +
                                      (modelData.uploaded ? "  ·  " + page.shortDate(modelData.uploaded) : "")
                                color: Tokens.inkFaint
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fTiny
                            }
                        }
                        Item { Layout.fillWidth: true }
                        Btn {
                            compact: true
                            text: I18n.tr("Install")
                            primary: modelData.primary
                            armed: !Daemon.modsBusy
                            onAct: {
                                filesSheet.shown = false;
                                page.installFile(modelData);
                            }
                        }
                    }
                }
            }

            RowLayout {
                Layout.fillWidth: true
                visible: page.hiddenFiles() > 0
                Text {
                    text: page.hiddenFiles() + " " + I18n.tr("archived builds hidden")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                }
                Item { Layout.fillWidth: true }
                Btn {
                    compact: true
                    text: I18n.tr("Show archived")
                    onAct: page.showArchived = true
                }
            }

            Text {
                Layout.fillWidth: true
                Layout.topMargin: Tokens.s2
                visible: Daemon.modsFilesBusy || (Daemon.modsFiles && Daemon.modsFiles.error)
                text: Daemon.modsFilesBusy ? I18n.tr("Asking Nexus for the file list…")
                     : (Daemon.modsFiles && Daemon.modsFiles.error ? Daemon.modsFiles.error : "")
                color: Daemon.modsFiles && Daemon.modsFiles.error ? Tokens.alert : Tokens.inkFaint
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }
            Item { Layout.fillHeight: true }
            RowLayout {
                Layout.fillWidth: true
                Item { Layout.fillWidth: true }
                Btn {
                    compact: true
                    text: I18n.tr("Close")
                    onAct: filesSheet.shown = false
                }
            }
        }
    }

    Sheet {
        id: keySheet
        title: I18n.tr("Nexus API key")
        subtitle: I18n.tr("nexusmods.com ▸ Account ▸ API. Hikari keeps the key in its own data directory.")
        onCloseRequested: keySheet.shown = false

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: Tokens.s6
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
                    placeholderText: I18n.tr("paste the key")
                    placeholderTextColor: Tokens.inkFaint
                    echoMode: TextInput.Password
                    selectByMouse: true
                }
            }
            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.s3
                Item { Layout.fillWidth: true }
                Btn {
                    compact: true
                    text: I18n.tr("Cancel")
                    onAct: keySheet.shown = false
                }
                Btn {
                    compact: true
                    text: I18n.tr("Save")
                    primary: true
                    armed: keyField.text.length > 8
                    onAct: {
                        Daemon.setNexusKey(keyField.text.trim());
                        keyField.text = "";
                        keySheet.shown = false;
                        page.search(0);
                    }
                }
            }
            Item { Layout.fillHeight: true }
        }
    }
}
