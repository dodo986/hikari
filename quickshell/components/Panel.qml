import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons

Rectangle {
    id: root

    property string title: ""
    property string subtitle: ""
    default property alias content: body.data

    implicitHeight: body.implicitHeight + Tokens.s5 * 2
    radius: Tokens.radius + Tokens.px(2)
    color: Tokens.paperLift
    border.width: Tokens.border
    border.color: Tokens.lineSoft

    ColumnLayout {
        id: body
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: Tokens.s5
        spacing: Tokens.s3

        ColumnLayout {
            visible: root.title.length > 0 || root.subtitle.length > 0
            spacing: Tokens.s1
            Layout.fillWidth: true
            Text {
                visible: root.title.length > 0
                text: root.title
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fRow
                font.weight: Font.Medium
            }
            Text {
                visible: root.subtitle.length > 0
                text: root.subtitle
                color: Tokens.inkFaint
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
                Layout.fillWidth: true
            }
        }
    }
}
