import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Rectangle {
    id: sw

    property bool on: false
    signal toggled(bool v)

    activeFocusOnTab: true
    function activate() { toggled(!on) }
    Keys.onPressed: event => {
        if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
            activate();
            event.accepted = true;
        }
    }

    implicitWidth: 54
    implicitHeight: 24
    radius: Tokens.radius
    color: tap.pressed ? Tokens.tint16 : (sw.on ? Tokens.tint16 : Tokens.tint5)
    border.width: Tokens.border
    border.color: activeFocus ? Tokens.bone : (tap.containsMouse ? Tokens.lineStrong : Tokens.line)
    antialiasing: false
    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
    Behavior on color { ColorAnimation { duration: Tokens.snap } }

    Rectangle {
        width: 25
        height: 17
        y: 3
        x: sw.on ? parent.width - width - 3 : 3
        radius: Tokens.radius
        antialiasing: false
        color: sw.on ? Tokens.ink : Tokens.inkDim
        border.width: 0
        Behavior on x { NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }
    }

    MouseArea {
        id: tap
        anchors.fill: parent
        enabled: sw.enabled
        hoverEnabled: true
        acceptedButtons: Qt.LeftButton
        cursorShape: Qt.PointingHandCursor
        onClicked: sw.toggled(!sw.on)
    }
}
