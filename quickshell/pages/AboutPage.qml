import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"

PageBase {
    id: page

    readonly property string version: Daemon.daemonStatus ? Daemon.daemonStatus.version : "0.1.0"
    readonly property string github: "https://github.com/dodo986"

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: Tokens.s5
        spacing: Tokens.s4

        ColumnLayout {
            spacing: Tokens.s1
            Text {
                text: I18n.tr("About")
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fTitle
            }
            Text {
                text: I18n.tr("A game launcher for Ryoku.")
                color: Tokens.inkFaint
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
        }

        Panel {
            Layout.fillWidth: true
            title: "Hikari " + page.version + "  ·  GPL-3.0-or-later"
            subtitle: I18n.tr("Epic, GOG, Amazon, Steam and your own games, run through Wine or Proton.")

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.s3
                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s1
                    Text {
                        text: I18n.tr("Made by dodo986")
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }
                    Text {
                        Layout.fillWidth: true
                        text: I18n.tr("Source, issues and releases live on GitHub.")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny
                        wrapMode: Text.WordWrap
                    }
                }
                Rectangle {
                    implicitWidth: linkRow.implicitWidth + Tokens.s4 * 2
                    implicitHeight: Tokens.px(32)
                    radius: Tokens.radius + Tokens.px(2)
                    color: linkHover.containsMouse ? Tokens.tint10 : Tokens.tint5
                    border.width: Tokens.border
                    border.color: linkHover.containsMouse ? Tokens.lineStrong : Tokens.lineSoft
                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                    Row {
                        id: linkRow
                        anchors.centerIn: parent
                        spacing: Tokens.s2
                        Text {
                            text: "GitHub"
                            color: Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                        }
                        Text {
                            text: "@dodo986  ↗"
                            color: Tokens.sun
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                        }
                    }
                    MouseArea {
                        id: linkHover
                        anchors.fill: parent
                        hoverEnabled: true
                        acceptedButtons: Qt.LeftButton
                        cursorShape: Qt.PointingHandCursor
                        onClicked: Qt.openUrlExternally(page.github)
                    }
                }
            }
        }

        Panel {
            Layout.fillWidth: true
            title: I18n.tr("References")
            subtitle: I18n.tr("What Hikari is built on and what it learned from.")

            Text {
                Layout.fillWidth: true
                text: I18n.tr("Heroic Games Launcher — store behaviour, launch pipeline and settings semantics.")
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }
            Text {
                Layout.fillWidth: true
                text: I18n.tr("Ryoku.Ui — design tokens and components.")
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }
            Text {
                Layout.fillWidth: true
                text: I18n.tr("OptiScaler — upscaler integration, implemented clean-room.")
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }
        }

        Item { Layout.fillHeight: true }
    }
}
