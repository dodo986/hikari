import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Rectangle {
    id: root

    property bool busy: false
    property bool armed: true
    signal act()

    activeFocusOnTab: armed

    implicitWidth: row.implicitWidth + Tokens.s4 * 2
    implicitHeight: Tokens.px(32)
    radius: Tokens.radius
    opacity: armed ? 1 : 0.55
    color: tap.pressed && armed ? Tokens.tint16 : (hover.hovered && armed ? Tokens.tint10 : Tokens.tint5)
    border.width: activeFocus ? Tokens.border * 2 : Tokens.border
    border.color: activeFocus ? Tokens.sun : Tokens.line
    Behavior on color { ColorAnimation { duration: Tokens.snap } }
    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

    Row {
        id: row
        anchors.centerIn: parent
        spacing: Tokens.s2
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "↻"
            color: root.busy ? Tokens.sun : Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            RotationAnimation on rotation {
                from: 0
                to: 360
                duration: Tokens.durXl
                loops: Animation.Infinite
                running: root.busy
            }
            onRotationChanged: if (!root.busy && rotation !== 0) rotation = 0
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: root.busy ? I18n.tr("Refreshing…") : I18n.tr("Refresh")
            color: root.busy ? Tokens.sun : Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            font.weight: Font.Medium
        }
    }

    Keys.onPressed: event => {
        if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
            if (root.armed && !root.busy) root.act();
            event.accepted = true;
        }
    }
    HoverHandler { id: hover; enabled: root.armed; cursorShape: Qt.PointingHandCursor }
    TapHandler {
        id: tap
        enabled: root.armed && !root.busy
        onTapped: root.act()
    }
}
