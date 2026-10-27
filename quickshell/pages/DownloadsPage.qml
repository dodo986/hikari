import QtQuick
import QtQuick.Controls.Basic
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"

PageBase {
    id: page

    readonly property var rows: {
        var list = [];
        for (var key in Daemon.installs) list.push(Daemon.installs[key]);
        return list;
    }

    function gameFor(item) { return item ? Daemon.findGame(item.store, item.id) : null; }
    function titleFor(item) {
        var g = gameFor(item);
        return g ? g.title : (item ? item.id : "");
    }

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
                    text: I18n.tr("Downloads")
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fTitle
                }
                Text {
                    text: page.rows.length + " " + I18n.tr("in flight") +
                          "  ·  " + Daemon.runningGames.length + " " + I18n.tr("playing")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }
            }
            Item { Layout.fillWidth: true }
            Badge {
                visible: page.rows.length + Daemon.runningGames.length > 0
                text: I18n.tr("ACTIVE")
                accent: true
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
                    spacing: Tokens.s4

                    Text {
                        visible: Daemon.runningGames.length > 0
                        text: I18n.tr("PLAYING NOW")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny
                        font.letterSpacing: Tokens.trackMark
                    }
                    Repeater {
                        model: Daemon.runningGames
                        Entrance {
                            required property var modelData
                            Layout.fillWidth: true
                            Panel {
                                width: parent.width
                                RowLayout {
                                    Layout.fillWidth: true
                                    spacing: Tokens.s3
                                    GameArt {
                                        Layout.preferredWidth: Tokens.px(64)
                                        Layout.preferredHeight: Tokens.px(84)
                                        game: page.gameFor(modelData)
                                        kind: "portrait"
                                        showTitle: false
                                        radius: Tokens.radius
                                        border.width: Tokens.border
                                        border.color: Tokens.lineSoft
                                    }
                                    ColumnLayout {
                                        spacing: Tokens.s1
                                        Text {
                                            text: page.titleFor(modelData)
                                            color: Tokens.ink
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fRow
                                            font.weight: Font.Medium
                                        }
                                        Text {
                                            text: modelData.store.toUpperCase() + "  ·  " +
                                                  (Daemon.stateOf(modelData.store, modelData.id) === "launching"
                                                   ? I18n.tr("starting") : I18n.tr("running")) +
                                                  (modelData.pid ? "  ·  pid " + modelData.pid : "")
                                            color: Tokens.inkFaint
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                        }
                                    }
                                    Item { Layout.fillWidth: true }
                                    Btn {
                                        compact: true
                                        stealFocus: true
                                        text: I18n.tr("Stop")
                                        onAct: Daemon.call("game.stop", {
                                            store: modelData.store, id: modelData.id }, null)
                                    }
                                }
                            }
                        }
                    }

                    Text {
                        visible: page.rows.length > 0
                        text: I18n.tr("DOWNLOADS")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny
                        font.letterSpacing: Tokens.trackMark
                    }
                    Repeater {
                        model: page.rows
                        Entrance {
                            required property var modelData
                            Layout.fillWidth: true
                            Panel {
                                width: parent.width
                                RowLayout {
                                    Layout.fillWidth: true
                                    spacing: Tokens.s3
                                    GameArt {
                                        Layout.preferredWidth: Tokens.px(56)
                                        Layout.preferredHeight: Tokens.px(74)
                                        game: page.gameFor(modelData)
                                        kind: "portrait"
                                        showTitle: false
                                        radius: Tokens.radius
                                        border.width: Tokens.border
                                        border.color: Tokens.lineSoft
                                    }
                                    ColumnLayout {
                                        Layout.fillWidth: true
                                        spacing: Tokens.s2
                                        RowLayout {
                                            Layout.fillWidth: true
                                            spacing: Tokens.s2
                                            Text {
                                                Layout.fillWidth: true
                                                text: page.titleFor(modelData)
                                                color: Tokens.ink
                                                font.family: Tokens.ui
                                                font.pixelSize: Tokens.fRow
                                                font.weight: Font.Medium
                                                elide: Text.ElideRight
                                            }
                                            Badge { text: modelData.state; accent: true }
                                        }
                                        Rectangle {
                                            Layout.fillWidth: true
                                            implicitHeight: Tokens.px(5)
                                            radius: Tokens.px(2)
                                            color: Tokens.tint10
                                            Rectangle {
                                                height: parent.height
                                                radius: parent.radius
                                                color: Tokens.sun
                                                width: parent.width * ((modelData.percent !== null && modelData.percent !== undefined)
                                                                       ? modelData.percent / 100 : 0)
                                                Behavior on width { NumberAnimation { duration: Tokens.move } }
                                            }
                                        }
                                        Text {
                                            text: (modelData.percent !== null && modelData.percent !== undefined
                                                   ? modelData.percent.toFixed(1) + "%  ·  " : "") +
                                                  (modelData.bytes || "") +
                                                  (modelData.eta ? "  ·  " + I18n.tr("eta") + " " + modelData.eta : "") +
                                                  (modelData.down_speed ? "  ·  " + modelData.down_speed.toFixed(1) + " MB/s" : "")
                                            color: Tokens.inkFaint
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                            elide: Text.ElideRight
                                        }
                                    }
                                    Btn {
                                        compact: true
                                        stealFocus: true
                                        text: I18n.tr("Cancel")
                                        onAct: Daemon.call("game.install.cancel", {
                                            store: modelData.store, id: modelData.id }, null)
                                    }
                                }
                            }
                        }
                    }

                }
            }
            Empty {
                anchors.centerIn: parent
                visible: page.rows.length === 0 && Daemon.runningGames.length === 0
                caption: I18n.tr("Nothing downloading. Installs and running games appear here.")
            }
        }

    }
}
