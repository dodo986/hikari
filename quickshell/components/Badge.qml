import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Rectangle {
    id: root

    property alias text: label.text
    property bool accent: false
    property bool solid: false
    property bool muted: false
    property bool danger: false

    implicitWidth: label.implicitWidth + Tokens.s3 * 2
    implicitHeight: Tokens.px(24)
    
    radius: Tokens.radius + Tokens.px(2)
    color: root.solid ? Tokens.bone : (root.danger ? Qt.rgba(Tokens.alert.r, Tokens.alert.g, Tokens.alert.b, 0.14) : (root.accent ? Tokens.tint16 : Tokens.tint5))
    border.width: Tokens.border
    border.color: root.solid ? Tokens.bone : (root.danger ? Tokens.alert : (root.accent ? Tokens.sun : Tokens.lineSoft))
    Behavior on color { ColorAnimation { duration: Tokens.snap } }
    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

    Text {
        id: label
        anchors.centerIn: parent
        color: root.solid ? Tokens.inkOnBone : (root.danger ? Tokens.alert : (root.accent ? Tokens.sun : (root.muted ? Tokens.inkFaint : Tokens.inkDim)))
        font.family: Tokens.ui
        font.pixelSize: Tokens.fTiny
        font.weight: Font.Medium
        font.letterSpacing: Tokens.trackLabel
    }
}
