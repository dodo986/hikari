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
    signal openGame(string store, string id)

    property string query: ""
    property string store: "all"
    property bool storeMode: false
    property string sortBy: "title"
    property bool refreshing: false
    property string refreshNote: ""
    property bool refreshError: false
    property bool refreshWarning: false
    property bool addSheetOpen: false
    property string addNote: ""
    property bool addError: false
    property string addCover: ""
    property var addCandidates: []
    property var addCovers: []
    property string coverNote: ""

    function coverSource(cover) {
        if (!cover) return "";
        return cover.indexOf("://") >= 0 ? cover : "file://" + cover;
    }

    function searchCovers() {
        var title = addTitle.text.trim();
        if (title.length < 2 || page.coverSearching) return;
        page.coverSearching = true;
        page.coverNote = I18n.tr("searching covers…");
        Daemon.call("artwork.search", { title: title }, (r, e) => {
            page.coverSearching = false;
            if (e) {
                page.coverNote = e.message;
                page.addCandidates = [];
                page.addCovers = [];
                return;
            }
            page.addCandidates = r.candidates || [];
            var covers = [];
            for (var i = 0; i < page.addCandidates.length; i++) {
                var cs = page.addCandidates[i].covers || [];
                for (var j = 0; j < cs.length; j++) covers.push(cs[j]);
            }
            page.addCovers = covers;
            page.coverNote = covers.length
                ? I18n.tr("pick a cover")
                : I18n.tr("no covers found");
            if (covers.length > 0 && page.addCover === "")
                page.addCover = covers[0].url;
        });
    }
    property bool coverSearching: false

    function addLocalGame() {
        var title = addTitle.text.trim();
        var exe = addExe.text.trim();
        if (title === "" || exe === "") {
            page.addError = true;
            page.addNote = I18n.tr("A title and an absolute executable path are required");
            return;
        }
        var params = {
            title: title, executable: exe,
            platform: addWindows.selected ? "Windows" : "Linux"
        };
        if (page.addCover !== "") params.artwork = page.addCover;
        Daemon.call("game.add_local", params, (r, e) => {
            if (e) {
                page.addError = true;
                page.addNote = e.message;
                return;
            }
            page.addSheetOpen = false;
            page.addNote = "";
            page.addError = false;
            addTitle.text = "";
            addExe.text = "";
            page.addCover = "";
            page.addCovers = [];
            page.addCandidates = [];
            page.coverNote = "";
            Daemon.refreshLibrary();
        });
    }

    readonly property int tileW: Tokens.px(220)
    readonly property int tileH: Math.round(tileW * 4 / 3)
    readonly property int tileMetaH: Tokens.px(68)
    readonly property int tileCellH: tileH + tileMetaH
    readonly property int availableWidth: Math.max(0, page.width - Tokens.s5 * 2)
    readonly property int columns: Math.max(1, Math.floor((availableWidth + Tokens.s4) / (tileW + Tokens.s4)))

    readonly property var storeChips: [
        { key: "all",     label: I18n.tr("All") },
        { key: "epic",    label: I18n.tr("Epic") },
        { key: "gog",     label: I18n.tr("GOG") },
        { key: "amazon",  label: I18n.tr("Amazon") },
        { key: "steam",   label: I18n.tr("Steam") },
        { key: "heroic",  label: I18n.tr("From Heroic") },
        { key: "local",   label: I18n.tr("Local") }
    ]

    readonly property int installedCount: {
        var count = 0;
        for (var i = 0; i < Daemon.games.length; i++)
            if (Daemon.games[i].installed) count++;
        return count;
    }

    readonly property int storeCount: Daemon.games.length - installedCount

    function countFor(store) {
        var count = 0;
        for (var i = 0; i < Daemon.games.length; i++) {
            var g = Daemon.games[i];
            if (store !== "all" && g.store !== store) continue;
            if (!page.inPage(g)) continue;
            count++;
        }
        return count;
    }

    function inPage(g) {
        return page.storeMode ? g.installed !== true : g.installed === true;
    }

    function timeValue(s) {
        if (!s) return 0;
        var v = Date.parse(String(s).replace(/(\.\d{3})\d+/, "$1"));
        return isNaN(v) ? 0 : v;
    }

    readonly property var slotIndex: {
        var m = {};
        for (var i = 0; i < page.shown.length; i++) {
            var g = page.shown[i];
            m[g.store + "/" + g.id] = i;
        }
        return m;
    }
    function slotFor(key) {
        var v = page.slotIndex[key];
        return v === undefined ? -1 : v;
    }

    function refreshLibrary() {
        if (page.refreshing) return;
        page.refreshing = true;
        page.refreshNote = "";
        Daemon.call("library.refresh", {}, (r, e) => {
            page.refreshing = false;
            if (e) {
                page.refreshNote = e.message;
                page.refreshError = true;
                page.refreshWarning = false;
                return;
            }
            var total = 0;
            var failed = 0;
            if (r && r.refreshed) {
                for (var key in r.refreshed) total += r.refreshed[key];
            }
            if (r && r.errors) {
                failed = Object.keys(r.errors).length;
            }
            page.refreshError = total === 0 && failed > 0;
            page.refreshWarning = failed > 0 && !page.refreshError;
            page.refreshNote = I18n.tr("Refreshed") + " " + total + " " + I18n.tr("titles") +
                (failed > 0 ? "  ·  " + failed + " " + I18n.tr("store unavailable") : "");
            Daemon.refreshAll();
            noteTimer.restart();
        });
    }

    readonly property var shown: {
        var list = [];
        for (var i = 0; i < Daemon.games.length; i++) {
            var g = Daemon.games[i];
            if (page.store !== "all" && g.store !== page.store) continue;
            if (!page.inPage(g)) continue;
            if (page.query.length > 0 &&
                    g.title.toLowerCase().indexOf(page.query.toLowerCase()) < 0) continue;
            list.push(g);
        }
        var by = page.sortBy;
        list.sort((a, b) => {
            if (by === "recent") {
                var ka = Daemon.isRunning(a.store, a.id) ? 1 : 0;
                var kb = Daemon.isRunning(b.store, b.id) ? 1 : 0;
                if (ka !== kb) return kb - ka;
                var ta = page.timeValue(a.last_played);
                var tb = page.timeValue(b.last_played);
                if (ta !== tb) return tb - ta;
            }
            return a.title.localeCompare(b.title);
        });
        return list;
    }

    Timer {
        id: noteTimer
        interval: Tokens.durXl * 4
        onTriggered: {
            page.refreshNote = "";
            page.refreshError = false;
            page.refreshWarning = false;
        }
    }

    ListModel { id: gridModel }

    function syncGrid() {
        var games = Daemon.games;
        var wanted = {};
        for (var i = 0; i < games.length; i++)
            wanted[games[i].store + "/" + games[i].id] = true;

        for (var i = gridModel.count - 1; i >= 0; i--) {
            var row = gridModel.get(i);
            if (!row || !wanted[row.entry.store + "/" + row.entry.id])
                gridModel.remove(i);
        }

        var at = {};
        for (var i = 0; i < gridModel.count; i++) {
            var e = gridModel.get(i).entry;
            at[e.store + "/" + e.id] = i;
        }
        for (var j = 0; j < games.length; j++) {
            var key = games[j].store + "/" + games[j].id;
            var idx = at[key];
            if (idx === undefined) {
                gridModel.append({ entry: games[j] });
                at[key] = gridModel.count - 1;
            } else if (JSON.stringify(gridModel.get(idx).entry) !== JSON.stringify(games[j])) {
                gridModel.setProperty(idx, "entry", games[j]);
            }
        }
    }

    function clampScroll() {
        gridScroll.contentY = Math.min(gridScroll.contentY,
                                       Math.max(0, gridScroll.contentHeight - gridScroll.height));
    }
    
    Connections {
        target: Daemon
        function onGamesChanged() { page.syncGrid() }
    }
    onShownChanged: page.clampScroll()
    Component.onCompleted: page.syncGrid()

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: Tokens.s5
        spacing: Tokens.s4

        RowLayout {
            Layout.fillWidth: true
            spacing: Tokens.s4

            ColumnLayout {
                spacing: Tokens.s1
                Text {
                    text: page.storeMode ? I18n.tr("Store") : I18n.tr("Library")
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fTitle
                }
                Text {
                    text: page.refreshNote.length > 0 ? page.refreshNote
                          : (page.storeMode
                             ? (page.storeCount + " " + I18n.tr("available to install") +
                                "  ·  " + page.installedCount + " " + I18n.tr("installed"))
                             : (page.installedCount + " " + I18n.tr("installed") +
                                "  ·  " + page.storeCount + " " + I18n.tr("in the store")))
                    color: page.refreshError ? Tokens.alert
                          : (page.refreshWarning ? Tokens.sunDeep : Tokens.inkFaint)
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                }
            }
            Item { Layout.fillWidth: true }
            Field {
                id: search
                Layout.preferredWidth: Tokens.px(280)
                placeholder: I18n.tr("Search games")
                onEdited: v => page.query = v
            }
            ActionButton {
                visible: !page.storeMode
                compact: true
                glyph: "+"
                text: I18n.tr("Add game")
                onAct: page.addSheetOpen = true
            }
            RefreshButton {
                busy: page.refreshing
                onAct: page.refreshLibrary()
            }
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: Tokens.s2
            Repeater {
                model: page.storeChips
                Btn {
                    required property var modelData
                    visible: modelData.key === "all" || page.countFor(modelData.key) > 0
                    compact: true
                    armed: page.store !== modelData.key
                    primary: page.store === modelData.key
                    text: modelData.label + "  " + page.countFor(modelData.key)
                    onAct: page.store = modelData.key
                }
            }
            Item { Layout.fillWidth: true }
            Text {
                text: I18n.tr("Sort")
                color: Tokens.inkFaint
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
            Btn {
                compact: true
                armed: page.sortBy !== "title"
                primary: page.sortBy === "title"
                text: I18n.tr("A–Z")
                onAct: page.sortBy = "title"
            }
            Btn {
                compact: true
                armed: page.sortBy !== "recent"
                primary: page.sortBy === "recent"
                text: I18n.tr("Playing first")
                onAct: page.sortBy = "recent"
            }
        }

        Rectangle {
            Layout.fillWidth: true
            height: 1
            color: Tokens.lineSoft
        }

        // Tiles persist and use the slot map; GridView has no move transition.
        Flickable {
            id: gridScroll
            Layout.fillWidth: true
            Layout.fillHeight: true
            contentWidth: width
            contentHeight: gridContent.height
            clip: true
            interactive: !page.addSheetOpen
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollBar {}

            Item {
                id: gridContent
                width: Math.min(gridScroll.width, page.columns * cellW)
                readonly property int cellW: page.tileW + Tokens.s4
                readonly property int cellH: page.tileCellH + Tokens.s4
                readonly property int rows: Math.ceil(page.shown.length / page.columns)
                height: Math.max(gridScroll.height, rows * cellH)

                Repeater {
                    model: gridModel

                    delegate: Tile {
                        required property var entry
                        required property int index

                        readonly property string tileKey: entry.store + "/" + entry.id
                        readonly property int slot: page.slotFor(tileKey)
                        readonly property bool placed: slot >= 0
                        
                        // A filtered-out tile keeps its last cell while it fades.
                        property int lastSlot: slot >= 0 ? slot : 0
                        
                        property bool settled: false
                        Timer {
                            id: settleTimer
                            interval: 50
                            onTriggered: parent.settled = true
                        }
                        onSlotChanged: {
                            if (slot < 0) return;
                            lastSlot = slot;
                            if (!settled) settleTimer.restart();
                        }
                        Component.onCompleted: if (slot >= 0) settleTimer.start()

                        game: entry
                        present: placed
                        revealIndex: index
                        x: ((placed ? slot : lastSlot) % page.columns) * gridContent.cellW
                        y: Math.floor((placed ? slot : lastSlot) / page.columns) * gridContent.cellH

                        Behavior on x {
                            enabled: settled
                            NumberAnimation {
                                duration: 440
                                easing.type: Easing.OutBack
                                easing.overshoot: 2.0
                            }
                        }
                        Behavior on y {
                            enabled: settled
                            NumberAnimation {
                                duration: 440
                                easing.type: Easing.OutBack
                                easing.overshoot: 2.0
                            }
                        }

                        onOpen: page.openGame(entry.store, entry.id)
                    }
                }
            }

            Text {
                anchors.centerIn: parent
                visible: Daemon.connected && Daemon.games.length > 0 && page.shown.length === 0
                text: (page.query.length > 0 || page.store !== "all")
                    ? I18n.tr("No games match these filters")
                    : (page.storeMode
                        ? I18n.tr("Everything you own is installed.")
                        : I18n.tr("Nothing installed yet. Pick a game in the Store."))
                color: Tokens.inkFaint
                font.family: Tokens.ui
                font.pixelSize: Tokens.fBody
            }
            Empty {
                anchors.centerIn: parent
                visible: Daemon.connected && Daemon.games.length === 0
                caption: I18n.tr("Nothing here yet. Log in to a store in Accounts, or install Heroic games and Hikari will find them.")
            }
        }
    }
    Sheet {
        id: addSheet
        shown: page.addSheetOpen
        title: I18n.tr("Add a local game")
        subtitle: I18n.tr("Point at an executable Hikari did not get from a store. Windows programs run through the game's compatibility layer.")
        onCloseRequested: page.addSheetOpen = false

        RowLayout {
            Layout.fillWidth: true
            spacing: Tokens.s4

            ColumnLayout {
                spacing: Tokens.s2
                Rectangle {
                    Layout.preferredWidth: Tokens.px(168)
                    Layout.preferredHeight: Tokens.px(252)
                    radius: Tokens.radius + Tokens.px(2)
                    color: Tokens.paperLift
                    border.width: Tokens.border
                    border.color: Tokens.line
                    clip: true
                    Image {
                        anchors.fill: parent
                        anchors.margins: Tokens.border + 1
                        source: page.coverSource(page.addCover)
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                    }
                    Text {
                        visible: page.addCover === ""
                        anchors.centerIn: parent
                        width: parent.width - Tokens.s4
                        text: I18n.tr("type a title to fetch covers")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny
                        horizontalAlignment: Text.AlignHCenter
                        wrapMode: Text.WordWrap
                    }
                }
                Text {
                    Layout.preferredWidth: Tokens.px(168)
                    text: page.coverNote
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                    wrapMode: Text.WordWrap
                }
                Flow {
                    Layout.preferredWidth: Tokens.px(168)
                    spacing: Tokens.s1
                    Repeater {
                        model: page.addCovers
                        Rectangle {
                            id: coverTile
                            required property var modelData
                            width: Tokens.px(48)
                            height: Tokens.px(72)
                            radius: Tokens.radius
                            color: Tokens.paperLift
                            border.width: page.addCover === modelData.url ? Tokens.border * 2 : Tokens.border
                            border.color: page.addCover === modelData.url ? Tokens.sun : Tokens.line
                            Image {
                                anchors.fill: parent
                                anchors.margins: 1
                                source: page.coverSource(coverTile.modelData.thumb || coverTile.modelData.url)
                                fillMode: Image.PreserveAspectCrop
                                asynchronous: true
                            }
                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                onClicked: page.addCover = coverTile.modelData.url
                            }
                        }
                    }
                }
            }

            ColumnLayout {
                Layout.fillWidth: true
                spacing: Tokens.s3

                RowLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s3
                    Text {
                        text: I18n.tr("Title")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        Layout.preferredWidth: Tokens.px(84)
                    }
                    Field {
                        id: addTitle
                        Layout.fillWidth: true
                        placeholder: I18n.tr("the game's name")
                        onEdited: coverSearch.restart()
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s3
                    Text {
                        text: I18n.tr("Executable")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        Layout.preferredWidth: Tokens.px(84)
                    }
                    Field {
                        id: addExe
                        Layout.fillWidth: true
                        placeholder: I18n.tr("/absolute/path/to/game.exe")
                    }
                    ActionButton {
                        compact: true
                        text: I18n.tr("Browse…")
                        onAct: exePicker.open()
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s3
                    Text {
                        text: I18n.tr("Runs with")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        Layout.preferredWidth: Tokens.px(84)
                    }
                    ActionButton {
                        compact: true
                        primary: !addWindows.selected
                        text: I18n.tr("Linux (native)")
                        onAct: addWindows.selected = false
                    }
                    ActionButton {
                        id: addWindows
                        compact: true
                        property bool selected: true
                        primary: selected
                        text: I18n.tr("Windows (Wine/Proton)")
                        onAct: selected = true
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s3
                    Text {
                        text: I18n.tr("Cover")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        Layout.preferredWidth: Tokens.px(84)
                    }
                    ActionButton {
                        compact: true
                        text: I18n.tr("Search covers")
                        onAct: page.searchCovers()
                    }
                    ActionButton {
                        compact: true
                        text: I18n.tr("Choose file…")
                        onAct: coverPicker.open()
                    }
                    Item { Layout.fillWidth: true }
                }
                RowLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s3
                    ActionButton {
                        primary: true
                        text: I18n.tr("Add to library")
                        onAct: page.addLocalGame()
                    }
                    Text {
                        Layout.fillWidth: true
                        text: page.addNote
                        color: page.addError ? Tokens.alert : Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.WordWrap
                    }
                }
            }
        }

        Timer {
            id: coverSearch
            interval: 450
            onTriggered: page.searchCovers()
        }
    }

    FileDialogHost {
        id: exePicker
        title: I18n.tr("Choose the game executable")
        currentFolder: "file://" + (Quickshell.env("HOME") || "/")
        nameFilters: [I18n.tr("Programs (*.exe *.sh *.AppImage *)"), I18n.tr("All files (*)")]
        onAccepted: addExe.text = decodeURIComponent(exePicker.selectedFile.toString().replace("file://", ""))
    }
    FileDialogHost {
        id: coverPicker
        title: I18n.tr("Choose a cover image")
        currentFolder: "file://" + (Quickshell.env("HOME") || "/")
        nameFilters: [I18n.tr("Images (*.png *.jpg *.jpeg *.webp)")]
        onAccepted: page.addCover = decodeURIComponent(coverPicker.selectedFile.toString().replace("file://", ""))
    }

}
