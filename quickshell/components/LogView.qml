import QtQuick
import QtQuick.Layouts
import QtQuick.Controls.Basic
import Ryoku.Ui
import Ryoku.Ui.Singletons

Flickable {
    id: root

    property string text: ""
    property int preferredHeight: Tokens.px(220)

    function copyAll() {
        body.selectAll();
        body.copy();
    }

    Layout.fillWidth: true
    Layout.preferredHeight: preferredHeight
    contentWidth: width
    contentHeight: body.implicitHeight
    clip: true
    boundsBehavior: Flickable.StopAtBounds
    ScrollBar.vertical: ScrollBar {}

    TextEdit {
        id: body
        width: parent.width
        readOnly: true
        selectByMouse: true
        wrapMode: TextEdit.WrapAnywhere
        color: Tokens.inkDim
        font.family: Tokens.mono
        font.pixelSize: Tokens.fTiny
        text: root.text
    }
}
