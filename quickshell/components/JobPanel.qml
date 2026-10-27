import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "."

// The running mods job: phase counter, log tail, cancel. Shared by both mods views.
Panel {
    id: root

    // Set by callers whose job needs the owner to do something outside Hikari.
    property string actionText: ""
    signal action()

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
            Behavior on width { NumberAnimation { duration: Tokens.durNormal } }
        }
    }

    RowLayout {
        Layout.fillWidth: true
        spacing: Tokens.s3
        Text {
            Layout.fillWidth: true
            text: Daemon.modsLog.length > 0 ? Daemon.modsLog[Daemon.modsLog.length - 1]
                                            : I18n.tr("Working; the game must not be running.")
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fTiny
            elide: Text.ElideRight
        }
        Btn {
            compact: true
            visible: root.actionText !== ""
            text: root.actionText
            onAct: root.action()
        }
        Btn {
            compact: true
            text: I18n.tr("Cancel")
            onAct: Daemon.cancelMods()
        }
    }
}
