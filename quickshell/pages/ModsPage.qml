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

    property var chosen: ({})
    property bool showAll: false
    property string pendingGame: ""

    function profileFor(game) {
        if (page.chosen[game.id]) return page.chosen[game.id];
        if (game.deployed_profile) return game.deployed_profile;
        return game.profiles && game.profiles.length > 0 ? game.profiles[0] : "";
    }
    function cycleProfile(game) {
        var list = game.profiles || [];
        if (list.length < 2) return;
        var next = list[(list.indexOf(page.profileFor(game)) + 1) % list.length];
        var copy = Object.assign({}, page.chosen);
        copy[game.id] = next;
        page.chosen = copy;
    }
    function pickArchive(game) {
        page.pendingGame = game.id;
        archivePicker.open();
    }

    Connections {
        target: Daemon
        function onConnectedChanged() {
            if (Daemon.connected) Daemon.refreshMods();
        }
    }
    Component.onCompleted: Daemon.refreshMods()

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: Tokens.s5
        spacing: Tokens.s4

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
                    text: page.readyGames.length + " " + I18n.tr("ready to mod") +
                          "  ·  " + page.restGames.length + " " + I18n.tr("without a path in Amethyst")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }
            }
            Item { Layout.fillWidth: true }
            Badge {
                visible: page.status !== null
                text: page.ready ? I18n.tr("AMETHYST READY") : I18n.tr("BACKEND OFFLINE")
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
            title: I18n.tr("Amethyst backend unavailable")
            subtitle: page.status ? page.status.reason : ""
            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.s3
                Text {
                    Layout.fillWidth: true
                    text: I18n.tr("Hikari drives Amethyst's own modding backend. Install Amethyst, then let Hikari fetch its Python packages into its own data directory.")
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

        Panel {
            visible: Daemon.modsBusy
            Layout.fillWidth: true
            title: Daemon.modsProgress
                ? Daemon.modsProgress.phase + "  " + Daemon.modsProgress.current + "/" + Daemon.modsProgress.total
                : I18n.tr("Working…")
            subtitle: Daemon.modsProgress ? (Daemon.modsProgress.message || "") : ""
            Rectangle {
                Layout.fillWidth: true
                Layout.preferredHeight: Tokens.px(6)
                radius: Tokens.px(3)
                color: Tokens.line
                Rectangle {
                    height: parent.height
                    radius: parent.radius
                    color: Tokens.sun
                    width: parent.width * (Daemon.modsProgress && Daemon.modsProgress.total > 0
                                           ? Daemon.modsProgress.current / Daemon.modsProgress.total : 0)
                    Behavior on width { NumberAnimation { duration: Tokens.move } }
                }
            }
            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.s3
                Text {
                    Layout.fillWidth: true
                    text: Daemon.modsLog.length > 0
                        ? Daemon.modsLog[Daemon.modsLog.length - 1]
                        : I18n.tr("Amethyst is working on it; the game must not be running.")
                    color: Tokens.inkFaint
                    font.family: Daemon.modsLog.length > 0 ? Tokens.mono : Tokens.ui
                    font.pixelSize: Tokens.fTiny
                    elide: Text.ElideRight
                }
                Btn {
                    compact: true
                    text: I18n.tr("Cancel")
                    onAct: Daemon.cancelMods()
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
                            Panel {
                                width: parent.width
                                title: modelData.name
                                subtitle: modelData.store
                                    ? modelData.id + "  ·  " + modelData.store + "/" + modelData.game_id
                                    : modelData.id
                                RowLayout {
                                    Layout.fillWidth: true
                                    spacing: Tokens.s3
                                    Badge {
                                        text: page.profileFor(modelData) || I18n.tr("NO PROFILE")
                                        muted: !modelData.deploy_active
                                    }
                                    Badge {
                                        visible: modelData.deploy_active
                                        text: I18n.tr("DEPLOYED")
                                        accent: true
                                    }
                                    Item { Layout.fillWidth: true }
                                    Btn {
                                        compact: true
                                        visible: (modelData.profiles || []).length > 1
                                        text: I18n.tr("Profile")
                                        armed: !Daemon.modsBusy
                                        onAct: page.cycleProfile(modelData)
                                    }
                                    Btn {
                                        compact: true
                                        text: I18n.tr("Install archive")
                                        armed: !Daemon.modsBusy
                                        onAct: page.pickArchive(modelData)
                                    }
                                    Btn {
                                        compact: true
                                        visible: modelData.deploy_active
                                        text: I18n.tr("Restore")
                                        armed: !Daemon.modsBusy
                                        onAct: Daemon.restoreMods(modelData.id)
                                    }
                                    Btn {
                                        compact: true
                                        text: modelData.deploy_active
                                            ? I18n.tr("Deploy again") : I18n.tr("Deploy")
                                        primary: !modelData.deploy_active
                                        armed: !Daemon.modsBusy
                                        onAct: Daemon.deployMods(modelData.id, page.profileFor(modelData))
                                    }
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
                            text: page.showAll
                                ? I18n.tr("Hide them")
                                : I18n.tr("Show all") + " " + page.restGames.length
                            onAct: page.showAll = !page.showAll
                        }
                        Text {
                            Layout.fillWidth: true
                            text: I18n.tr("Amethyst has handlers for these too; set a game path there and it shows up here.")
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
                                Text {
                                    text: I18n.tr("no path in Amethyst")
                                    color: Tokens.inkFaint
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fTiny
                                }
                            }
                        }
                    }
                }
            }

            Empty {
                anchors.centerIn: parent
                visible: page.games.length === 0
                caption: page.ready
                    ? I18n.tr("Amethyst has no game handlers yet.")
                    : I18n.tr("The Amethyst backend is not available.")
            }
        }
    }

    FileDialogHost {
        id: archivePicker
        title: I18n.tr("Choose a mod archive")
        currentFolder: "file://" + (Quickshell.env("HOME") || "/")
        nameFilters: [I18n.tr("Archives (*.zip *.7z *.rar *.tar.gz *.tar.xz)"), I18n.tr("All files (*)")]
        onAccepted: {
            var path = decodeURIComponent(selectedFile.toString().replace("file://", ""));
            var game = page.games.find(g => g.id === page.pendingGame);
            if (game) Daemon.installMod(game.id, page.profileFor(game), path);
        }
    }
}
