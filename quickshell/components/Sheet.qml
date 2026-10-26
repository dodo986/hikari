import QtQuick
import QtQuick.Controls.Basic
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Do not use `parent: Overlay.overlay`: it is constant and null before the item has a window.
Item {
    id: sheet

    property string title: ""
    property string subtitle: ""
    property bool shown: false
    signal closeRequested()
    default property alias content: body.data

    visible: shown
    enabled: shown
    z: 1000
    focus: shown
    LayoutMirroring.enabled: I18n.rtl
    LayoutMirroring.childrenInherit: true
    width: parent ? parent.width : 0
    height: parent ? parent.height : 0
    Component.onCompleted: Qt.callLater(sheet.attach)
    onShownChanged: if (shown) {
        sheet.attach();
        sheet.forceActiveFocus();
    }

    function attach() {
        var win = sheet.Window.window;
        if (win && sheet.parent !== win.contentItem)
            sheet.parent = win.contentItem;
    }

    Keys.onEscapePressed: event => {
        sheet.closeRequested();
        event.accepted = true;
    }

    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.78)
        MouseArea {
            anchors.fill: parent
            onClicked: sheet.closeRequested()
        }
        
    // Do not add a root WheelHandler: it would eat the body Flickable's scrolling.
    WheelHandler {
            onWheel: event => event.accepted = true
        }
    }

    Rectangle {
        id: panel
        objectName: "sheetPanel"

        width: Math.min(parent.width - Tokens.s6 * 2, Tokens.px(780))
        height: Math.min(parent.height - Tokens.s6 * 2,
                         header.implicitHeight + body.implicitHeight +
                         Tokens.s3 * 2 + 1 + padding * 2)
        anchors.centerIn: parent
        radius: Tokens.radius + Tokens.px(2)
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        MouseArea { anchors.fill: parent; onClicked: {} }

        property int padding: Tokens.s4

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: panel.padding
            spacing: Tokens.s3

            RowLayout {
                id: header
                Layout.fillWidth: true
                spacing: Tokens.s3
                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s1
                    Text {
                        Layout.fillWidth: true
                        elide: Text.ElideRight
                        text: sheet.title
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fValue
                    }
                    Text {
                        visible: sheet.subtitle.length > 0
                        Layout.fillWidth: true
                        text: sheet.subtitle
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.WordWrap
                    }
                }
                Item { Layout.fillWidth: true }
                ActionButton {
                    objectName: "sheetClose"
                    compact: true
                    text: I18n.tr("Close")
                    onAct: sheet.closeRequested()
                }
            }

            Rectangle {
                Layout.fillWidth: true
                height: 1
                color: Tokens.lineSoft
            }

            Flickable {
                objectName: "sheetBody"
                Layout.fillWidth: true
                Layout.fillHeight: true
                implicitHeight: body.implicitHeight
                contentWidth: width
                contentHeight: body.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollBar {}

                ColumnLayout {
                    id: body
                    width: parent.width
                    spacing: Tokens.s3
                }
            }
        }
    }
}
