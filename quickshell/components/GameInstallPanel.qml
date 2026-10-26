import QtQuick
import Quickshell
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"

Entrance {
    id: root
    required property var page

    function installParams() {
        return {
            path: pathField.text,
            platform: (page.info && page.info.platform) || "Windows",
            dlcs: dlcRow.selected
        };
    }
    index: 0
    Layout.fillWidth: true
    
    // Hidden children still count in childrenRect, so the wrapper owns the visibility.
    visible: !!(page.info && !page.installed)
    Panel {
        width: parent.width
        title: I18n.tr("Install")
    subtitle: I18n.tr("The engine reports the real download and disk requirements before anything is written.")
    ColumnLayout {
        Layout.fillWidth: true
        spacing: Tokens.s3
        RowLayout {
            Layout.fillWidth: true
            spacing: Tokens.s3
            Text {
                text: I18n.tr("Location")
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
            Field {
                id: pathField
                Layout.fillWidth: true
                text: Quickshell.env("HOME") + "/Games/Hikari"
            }
        }
        Flow {
            visible: page.installInfo !== null
            Layout.fillWidth: true
            spacing: Tokens.s2
            Badge { text: I18n.tr("DOWNLOAD") + "  " + (page.installInfo ? page.fmtSize(page.installInfo.download_size) : "") }
            Badge { text: I18n.tr("ON DISK") + "  " + (page.installInfo ? page.fmtSize(page.installInfo.disk_size) : "") }
        }
        Flow {
            visible: page.installInfo && page.installInfo.dlcs && page.installInfo.dlcs.length > 0
            Layout.fillWidth: true
            spacing: Tokens.s2
            Repeater {
                model: page.installInfo && page.installInfo.dlcs ? page.installInfo.dlcs : []
                Btn {
                    required property var modelData
                    compact: true
                    property bool picked: dlcRow.selected.indexOf(modelData.id) >= 0
                    armed: !picked
                    primary: picked
                    text: modelData.title
                    onAct: dlcRow.toggle(modelData.id)
                }
            }
        }
        RowLayout {
            spacing: Tokens.s3
            ActionButton {
                compact: true
                primary: true
                text: I18n.tr("Start install")
                armed: pathField.text.length > 0
                onAct: page.install()
            }
            Text {
                visible: !!Daemon.installs[page.key]
                text: {
                    var p = Daemon.installs[page.key];
                    if (!p) return "";
                    return p.percent !== null && p.percent !== undefined
                        ? p.state + "  " + p.percent.toFixed(1) + "%" : p.state;
                }
                color: Tokens.sunDeep
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
        }
    }
}
}
