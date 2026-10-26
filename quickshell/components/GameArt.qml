import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Rectangle {
    id: root

    property var game: null
    property string role: "square"
    property bool showTitle: true
    property real artOpacity: 1.0

    readonly property bool ready: crop.ready
    readonly property url source: artworkUrl()

    function artworkUrl() {
        if (!root.game) return "";
        var local = root.game.artwork_local || ({});
        var remote = root.game.artwork || ({});
        var other = root.role === "cover" ? "square" : "cover";
        var value = local[root.role] || remote[root.role] ||
                    local[other] || remote[other] || "";
        if (!value) return "";
        return value.indexOf("://") < 0 ? "file://" + value : value;
    }

    color: Tokens.paperLift
    clip: true

    Rectangle {
        anchors.fill: parent
        visible: !root.ready
        gradient: Gradient {
            orientation: Gradient.Vertical
            GradientStop { position: 0.0; color: Tokens.tint16 }
            GradientStop { position: 1.0; color: Tokens.paperLift }
        }
        Text {
            visible: root.showTitle
            anchors.centerIn: parent
            width: Math.max(0, parent.width - Tokens.s5 * 2)
            horizontalAlignment: Text.AlignHCenter
            text: root.game && root.game.title ? root.game.title.charAt(0).toUpperCase() : "?"
            color: Tokens.inkFaint
            font.family: Tokens.display
            font.pixelSize: Math.min(Tokens.fHero * 2, parent.width / 3)
            font.weight: Font.Medium
        }
        Text {
            visible: root.showTitle && root.game && root.game.title
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.margins: Tokens.s3
            horizontalAlignment: Text.AlignHCenter
            text: root.game ? root.game.title : ""
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            font.weight: Font.Medium
            elide: Text.ElideRight
        }
    }

    HeroCrop {
        id: crop
        anchors.fill: parent
        source: root.source
        strength: root.artOpacity
        opacity: root.ready ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
    }
}
