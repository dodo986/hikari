import QtQuick
import QtQuick.Layouts
import QtTest
import "../components"

Item {
    id: root
    width: 900
    height: 640

    property int railClicks: 0
    property int tileClicks: 0
    property int closeRequests: 0
    property int bodyActs: 0

    Row {
        anchors.fill: parent

        Rectangle {
            id: rail
            width: 140
            height: parent.height
            color: "#222"
            MouseArea { anchors.fill: parent; onClicked: railClicks++ }
        }

        Flickable {
            id: pageArea
            objectName: "pageScroll"
            width: parent.width - rail.width
            height: parent.height
            contentHeight: 2000
            clip: true
            boundsBehavior: Flickable.StopAtBounds

            Loader {
                anchors.fill: parent
                sourceComponent: Component {
                    Item {
                        Rectangle {
                            anchors.fill: parent
                            color: "#333"
                            MouseArea { anchors.fill: parent; onClicked: tileClicks++ }
                        }
                        Sheet {
                            id: sheet
                            objectName: "sheet"
                            shown: true
                            title: "Test sheet"
                            onCloseRequested: closeRequests++

                            ActionButton {
                                objectName: "bodyButton"
                                compact: true
                                text: "Body button"
                                onAct: bodyActs++
                            }
                            Item { Layout.fillWidth: true; Layout.preferredHeight: 1400 }
                        }
                    }
                }
            }
        }
    }

    TestCase {
        name: "SheetInput"
        when: windowShown

        function sheetItem() { return findChild(root, "sheet") }

        function test_scrim_blocks_rail() {
            const before = closeRequests
            mouseClick(rail, 10, 10)
            compare(railClicks, 0, "rail received a click through the sheet")
            compare(closeRequests, before + 1, "scrim press should close")
        }

        function test_scrim_blocks_page() {
            const before = closeRequests
            mouseClick(pageArea, pageArea.width - 5, pageArea.height - 5)
            compare(tileClicks, 0, "page received a click through the sheet")
            compare(closeRequests, before + 1, "scrim press should close")
        }

        function test_close_button() {
            const sheet = sheetItem()
            const btn = findChild(sheet, "sheetClose")
            verify(btn !== null, "close button exists")
            const p = btn.mapToItem(pageArea, btn.width / 2, btn.height / 2)
            const before = closeRequests
            mouseClick(pageArea, p.x, p.y)
            compare(closeRequests, before + 1, "close button did not request close")
            compare(tileClicks, 0, "click below the close button reached the page")
        }

        function test_body_button() {
            const sheet = sheetItem()
            const btn = findChild(sheet, "bodyButton")
            verify(btn !== null, "body button exists")
            const p = btn.mapToItem(pageArea, btn.width / 2, btn.height / 2)
            const before = bodyActs
            mouseClick(pageArea, p.x, p.y)
            compare(bodyActs, before + 1, "body button did not activate")
            compare(tileClicks, 0, "click below the body button reached the page")
        }

        function test_wheel_scrolls_the_sheet_only() {
            const sheet = sheetItem()
            const body = findChild(sheet, "sheetBody")
            const panel = findChild(sheet, "sheetPanel")
            verify(body !== null && panel !== null, "sheet body and panel exist")
            body.contentY = 0
            pageArea.contentY = 0
            const p = panel.mapToItem(pageArea, panel.width / 2, panel.height * 0.75)
            mouseWheel(pageArea, p.x, p.y, 0, -120)
            wait(50)
            verify(body.contentY > 0, "wheel over the sheet did not scroll its body")
            compare(pageArea.contentY, 0, "wheel over the sheet scrolled the page behind")
        }

        function test_wheel_on_scrim_does_not_scroll_page() {
            pageArea.contentY = 0
            const sheet = sheetItem()
            const body = findChild(sheet, "sheetBody")
            body.contentY = 0
            mouseWheel(pageArea, pageArea.width - 4, pageArea.height - 4, 0, -120)
            compare(pageArea.contentY, 0, "wheel on the scrim scrolled the page behind")
        }

        function test_outside_panel_closes() {
            const before = closeRequests
            mouseClick(pageArea, 6, 6)
            compare(closeRequests, before + 1, "scrim outside the panel should close")
            compare(tileClicks, 0, "outside press leaked to the page")
        }

        function test_panel_padding_swallowed() {
            const sheet = sheetItem()
            const panel = findChild(sheet, "sheetPanel")
            const before = closeRequests
            mouseClick(pageArea, 0, 0)
            const p = panel.mapToItem(pageArea, 4, 4)
            mouseClick(pageArea, p.x, p.y)
            compare(closeRequests, before + 1, "first click closed; panel padding click must not add more")
            compare(tileClicks, 0, "panel padding press leaked")
        }
    }
}
