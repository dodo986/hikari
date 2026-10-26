import QtQuick
import QtQuick.Controls.Basic
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"
import "pages"
import "components"

Item {
    id: app

    property string page: "library"
    property var detail: null

    readonly property int installedCount: {
        var count = 0;
        for (var i = 0; i < Daemon.games.length; i++)
            if (Daemon.games[i].installed) count++;
        return count;
    }
    readonly property int storeCount: Daemon.games.length - installedCount
    readonly property int deployedCount: {
        var count = 0;
        for (var i = 0; i < Daemon.modsGames.length; i++)
            if (Daemon.modsGames[i].deploy_active) count++;
        return count;
    }

    function navigate(target: string) {
        if (target === "library" || target === "store" || target === "mods" ||
                target === "downloads" || target === "accounts" ||
                target === "settings" || target === "about") {
            app.detail = null;
            app.page = target;
        }
    }
    function openGame(store: string, id: string) { app.detail = { store: store, id: id }; }
    function closeDetail() { app.detail = null; }
    function requestQuit() { Qt.quit(); }

    focus: true
    Keys.onEscapePressed: event => {
        if (app.detail) { app.closeDetail(); }
        else event.accepted = false;
    }

    RowLayout {
        anchors.fill: parent
        spacing: 0

        Rectangle {
            Layout.fillHeight: true
            Layout.preferredWidth: Tokens.px(224)
            color: Tokens.paperLift

            ColumnLayout {
                anchors.fill: parent
                anchors.margins: Tokens.s4
                spacing: Tokens.s2

                ColumnLayout {
                    Layout.leftMargin: Tokens.s2
                    Layout.bottomMargin: Tokens.s4
                    spacing: Tokens.s1
                    Text {
                        text: "光 HIKARI"
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fBody
                        font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Text {
                        text: I18n.tr("GAME LAUNCHER")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny
                        font.letterSpacing: Tokens.trackMark
                    }
                }

                Item {
                    id: navBlock
                    Layout.fillWidth: true
                    implicitHeight: navColumn.implicitHeight

                    readonly property Item activeRow: app.detail ? null
                        : app.page === "library" ? rowLibrary
                        : app.page === "store" ? rowStore
                        : app.page === "mods" ? rowMods
                        : app.page === "downloads" ? rowDownloads
                        : app.page === "accounts" ? rowAccounts
                        : app.page === "settings" ? rowSettings
                        : app.page === "about" ? rowAbout : null

                    Column {
                        id: navColumn
                        width: parent.width
                        spacing: Tokens.s1

                        RailFold { label: I18n.tr("Browse") }
                        RailButton {
                            id: rowLibrary
                            glyph: "video_library"
                            label: I18n.tr("Library")
                            badge: app.installedCount
                            active: app.page === "library" && !app.detail
                            onClicked: { app.detail = null; app.page = "library"; }
                        }
                        RailButton {
                            id: rowStore
                            glyph: "storefront"
                            label: I18n.tr("Store")
                            badge: app.storeCount
                            active: app.page === "store" && !app.detail
                            onClicked: { app.detail = null; app.page = "store"; }
                        }
                        RailButton {
                            id: rowMods
                            glyph: "extension"
                            label: I18n.tr("Mods")
                            badge: app.deployedCount
                            active: app.page === "mods"
                            onClicked: { app.detail = null; app.page = "mods"; }
                        }
                        RailButton {
                            id: rowDownloads
                            glyph: "download"
                            label: I18n.tr("Downloads")
                            badge: Object.keys(Daemon.installs).length
                            active: app.page === "downloads"
                            onClicked: { app.detail = null; app.page = "downloads"; }
                        }

                        RailFold { label: I18n.tr("System") }
                        RailButton {
                            id: rowAccounts
                            glyph: "account_circle"
                            label: I18n.tr("Accounts")
                            active: app.page === "accounts"
                            onClicked: { app.detail = null; app.page = "accounts"; }
                        }
                        RailButton {
                            id: rowSettings
                            glyph: "settings"
                            label: I18n.tr("Settings")
                            active: app.page === "settings"
                            onClicked: { app.detail = null; app.page = "settings"; }
                        }
                        RailButton {
                            id: rowAbout
                            glyph: "info"
                            label: I18n.tr("About")
                            active: app.page === "about"
                            onClicked: { app.detail = null; app.page = "about"; }
                        }

                        RailFold {
                            visible: Daemon.runningGames.length > 0
                            label: I18n.tr("Playing")
                        }
                        Repeater {
                            model: Daemon.runningGames
                            RailButton {
                                required property var modelData
                                glyph: "play_arrow"
                                label: Daemon.findGame(modelData.store, modelData.id)
                                       ? Daemon.findGame(modelData.store, modelData.id).title : modelData.id
                                onClicked: app.openGame(modelData.store, modelData.id)
                            }
                        }
                    }

                    Rectangle {
                        id: navMarker
                        width: Tokens.px(5)
                        height: width
                        radius: Tokens.px(1.5)
                        x: Tokens.s1
                        color: Tokens.sun
                        opacity: navBlock.activeRow ? 1 : 0
                        y: navBlock.activeRow
                            ? navBlock.activeRow.y + navBlock.activeRow.height / 2 - height / 2
                            : navMarker.y
                        Behavior on y {
                            NumberAnimation { duration: 260; easing.type: Easing.OutCubic }
                        }
                        Behavior on opacity { NumberAnimation { duration: 120 } }
                    }
                }

                Item { Layout.fillHeight: true }

                ActionButton {
                    visible: Daemon.updateInfo !== null && (Daemon.updateInfo.available === true || Daemon.updateApplying || Daemon.updatePhase === "failed")
                    Layout.fillWidth: true
                    Layout.bottomMargin: Tokens.s3
                    compact: true
                    primary: true
                    glyph: "↑"
                    armed: !Daemon.updateApplying
                    text: Daemon.updateText()
                    onAct: Daemon.applyUpdate()
                }

                Rectangle {
                    visible: Daemon.connected
                    Layout.fillWidth: true
                    implicitHeight: Tokens.px(76)
                    radius: Tokens.radius
                    color: Tokens.tint5
                    border.width: Tokens.border
                    border.color: Tokens.lineSoft
                    ColumnLayout {
                        anchors.fill: parent
                        anchors.margins: Tokens.s3
                        spacing: Tokens.s1
                        Text {
                            text: I18n.tr("YOUR LIBRARY")
                            color: Tokens.inkFaint
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fTiny
                            font.letterSpacing: Tokens.trackMark
                        }
                        Text {
                            text: Daemon.games.length + " " + I18n.tr("games")
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fRow
                            font.weight: Font.Medium
                        }
                        Text {
                            text: app.installedCount + " " + I18n.tr("installed") +
                                  (Daemon.runningGames.length > 0 ? "  ·  " + Daemon.runningGames.length + " " + I18n.tr("playing") : "")
                            color: Tokens.inkFaint
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            elide: Text.ElideRight
                        }
                    }
                }
            }
            Rectangle {
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: 1
                color: Tokens.lineSoft
            }
        }

        Item {
            Layout.fillWidth: true
            Layout.fillHeight: true

            Loader {
                id: pager
                anchors.fill: parent
                opacity: 1
                transform: Translate { id: pageShift }
                sourceComponent: {
                    if (app.detail) return gameDetail;
                    switch (app.page) {
                    case "store": return storePage;
                    case "mods": return modsPage;
                    case "downloads": return downloadsPage;
                    case "accounts": return accountsPage;
                    case "settings": return settingsPage;
                    case "about": return aboutPage;
                    default: return libraryPage;
                    }
                }
                onLoaded: pageIn.restart()
            }

            ParallelAnimation {
                id: pageIn
                NumberAnimation {
                    target: pager
                    property: "opacity"
                    from: 0
                    to: 1
                    duration: Tokens.durFastSpatial
                    easing.type: Easing.Bezier
                    easing.bezierCurve: Tokens.curveFastSpatial
                }
                NumberAnimation {
                    target: pageShift
                    property: "y"
                    from: Tokens.s3
                    to: 0
                    duration: Tokens.durFastSpatial
                    easing.type: Easing.Bezier
                    easing.bezierCurve: Tokens.curveFastSpatial
                }
            }

            Component { id: libraryPage; LibraryPage { onOpenGame: (s, i) => app.openGame(s, i) } }
            Component { id: storePage; LibraryPage { storeMode: true; onOpenGame: (s, i) => app.openGame(s, i) } }
            Component { id: gameDetail; GameDetailPage { game: app.detail } }
            Component { id: modsPage; ModsPage {} }
            Component { id: downloadsPage; DownloadsPage {} }
            Component { id: accountsPage; AccountsPage {} }
            Component { id: settingsPage; SettingsPage {} }
            Component { id: aboutPage; AboutPage {} }

            Rectangle {
                anchors.fill: parent
                color: Tokens.paper
                visible: !Daemon.connected
                ColumnLayout {
                    anchors.centerIn: parent
                    spacing: Tokens.s4
                    Text {
                        Layout.alignment: Qt.AlignHCenter
                        text: I18n.tr("The Hikari daemon is not running")
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fTitle
                    }
                    Text {
                        Layout.alignment: Qt.AlignHCenter
                        text: I18n.tr("Games, logins and installs all come from hikarid. Start it, or run scripts/dev.sh from the repository.")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        horizontalAlignment: Text.AlignHCenter
                        Layout.maximumWidth: Tokens.contentMax
                    }
                    Btn {
                        Layout.alignment: Qt.AlignHCenter
                        text: I18n.tr("Start daemon")
                        primary: true
                        stealFocus: true
                        onAct: Daemon.startDaemon()
                    }
                }
            }
        }
    }

    component RailButton: Item {
        id: rb
        property string glyph: ""
        property string label: ""
        property bool active: false
        property int badge: 0
        signal clicked()

        width: parent ? parent.width : 0
        implicitHeight: Tokens.px(36)
        activeFocusOnTab: true

        Keys.onReturnPressed: rb.clicked()
        Keys.onEnterPressed: rb.clicked()
        Keys.onSpacePressed: rb.clicked()

        Rectangle {
            anchors.fill: parent
            radius: Tokens.radius * 1.5
            color: rb.active ? Tokens.tint16 : (rh.containsMouse ? Tokens.tint10 : "transparent")
            border.width: rb.activeFocus ? Tokens.border : 0
            border.color: Tokens.bone
            Behavior on color { ColorAnimation { duration: Tokens.snap } }

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: Tokens.s3
                anchors.rightMargin: Tokens.s3
                spacing: Tokens.s3
                Text {
                    Layout.preferredWidth: Tokens.px(20)
                    horizontalAlignment: Text.AlignHCenter
                    text: rb.glyph
                    color: rb.active ? Tokens.ink : Tokens.inkMuted
                    font.family: "Material Symbols Rounded"
                    font.pixelSize: Tokens.fValue
                    font.weight: Font.Medium
                }
                Text {
                    Layout.fillWidth: true
                    text: rb.label
                    color: rb.active ? Tokens.ink : Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fBody
                    font.weight: rb.active ? Font.Medium : Font.Normal
                    elide: Text.ElideRight
                }
                Text {
                    visible: rb.badge > 0
                    text: rb.badge
                    color: rb.active ? Tokens.inkDim : Tokens.inkFaint
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                }
            }
        }
        MouseArea {
            id: rh
            anchors.fill: parent
            hoverEnabled: true
            acceptedButtons: Qt.LeftButton
            cursorShape: Qt.PointingHandCursor
            onClicked: rb.clicked()
        }
    }

    component RailFold: Item {
        id: fold
        property string label: ""

        width: parent ? parent.width : 0
        implicitHeight: Tokens.fTiny + Tokens.s3 * 2

        Rectangle {
            anchors.left: parent.left
            anchors.right: mark.left
            anchors.rightMargin: Tokens.s2
            anchors.verticalCenter: parent.verticalCenter
            height: Tokens.border
            color: Tokens.lineSoft
        }
        Text {
            id: mark
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.verticalCenter: parent.verticalCenter
            text: fold.label.toUpperCase()
            color: Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
            font.letterSpacing: Tokens.trackLabel
        }
        Rectangle {
            anchors.left: mark.right
            anchors.leftMargin: Tokens.s2
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: Tokens.border
            color: Tokens.lineSoft
        }
    }
}
