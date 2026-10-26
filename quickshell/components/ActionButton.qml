import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Rectangle {
    id: root

    property alias text: label.text
    property string glyph: ""
    property bool primary: false
    property bool compact: false
    property bool armed: true
    property bool stealFocus: true
    signal act()

    activeFocusOnTab: armed

    implicitWidth: Math.max(compact ? Tokens.px(120) : Tokens.px(168),
                            row.implicitWidth + Tokens.s5 * 2)
    implicitHeight: compact ? Tokens.px(34) : Tokens.px(46)
    radius: Tokens.radius + Tokens.px(2)
    opacity: armed ? 1 : 0.45
    color: primary && armed ? Tokens.bone
                            : (tap.pressed && armed ? Tokens.tint16 : (tap.containsMouse && armed ? Tokens.tint10 : Tokens.tint5))
    border.width: activeFocus ? Tokens.border * 2 : Tokens.border
    border.color: activeFocus ? Tokens.sun : (primary && armed ? Tokens.bone : Tokens.line)
    scale: tap.pressed ? 0.985 : (tap.containsMouse && armed ? 1.01 : 1)
    Behavior on scale { NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }
    Behavior on color { ColorAnimation { duration: Tokens.snap } }
    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

    Row {
        id: row
        anchors.centerIn: parent
        spacing: Tokens.s2
        Text {
            visible: root.glyph.length > 0
            text: root.glyph
            anchors.verticalCenter: parent.verticalCenter
            color: root.primary && root.armed ? Tokens.inkOnBone : Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: root.compact ? Tokens.fSmall : Tokens.fBody
        }
        Text {
            id: label
            anchors.verticalCenter: parent.verticalCenter
            color: root.primary && root.armed ? Tokens.inkOnBone : Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: root.compact ? Tokens.fSmall : Tokens.fBody
            font.weight: Font.Medium
            font.letterSpacing: 0.4
        }
    }

    Keys.onPressed: event => {
        if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
            if (root.armed && !event.isAutoRepeat) root.act();
            event.accepted = true;
        }
    }
    
    // MouseArea: TapHandler presses never arrived inside a popout.
    MouseArea {
        id: tap
        anchors.fill: parent
        enabled: root.armed
        hoverEnabled: true
        acceptedButtons: Qt.LeftButton
        cursorShape: Qt.PointingHandCursor
        onPressed: if (root.stealFocus) root.forceActiveFocus()
        onClicked: root.act()
    }
}
