import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"

Item {
    id: tile

    required property var game
    signal open()

    property int revealIndex: 0
    property bool revealed: false
    property bool present: true

    readonly property int artW: Tokens.px(220)
    readonly property int artH: Math.round(artW * 4 / 3)
    readonly property int metaH: Tokens.px(68)
    readonly property int cellH: artH + metaH
    readonly property bool highlighted: tile.activeFocus || hover.hovered
    readonly property var progress: Daemon.installs[Daemon.gameKey(game.store, game.id)]

    width: tile.artW
    height: tile.cellH
    implicitWidth: width
    implicitHeight: height

    opacity: tile.revealed && tile.present ? 1 : 0
    visible: opacity > 0.01
    enabled: tile.present
    transform: Translate {
        y: tile.revealed ? 0 : Tokens.s3
        Behavior on y {
            NumberAnimation { duration: Tokens.durFastSpatial; easing.type: Tokens.ease }
        }
    }
    Behavior on opacity {
        NumberAnimation { duration: Tokens.durFastSpatial; easing.type: Tokens.ease }
    }
    Timer {
        id: revealTimer
        interval: Math.min(tile.revealIndex * Math.round(Tokens.snap / 3), Tokens.durFastSpatial)
        onTriggered: tile.revealed = true
    }
    Component.onCompleted: revealTimer.start()

    Keys.onReturnPressed: tile.open()
    Keys.onEnterPressed: tile.open()
    Keys.onSpacePressed: tile.open()

    Item {
        id: card
        width: tile.artW
        height: tile.artH
        y: tile.highlighted ? -Tokens.s1 : 0
        Behavior on y { NumberAnimation { duration: Tokens.move; easing.type: Tokens.easeSnap } }

        Rectangle {
            x: 0
            y: Tokens.s2
            width: card.width
            height: card.height
            radius: Tokens.radius + Tokens.px(3)
            color: Tokens.paper
            opacity: tile.highlighted ? 0.85 : 0
            Behavior on opacity { NumberAnimation { duration: Tokens.move } }
        }

        GameArt {
            anchors.fill: parent
            game: tile.game
            kind: "portrait"
            scale: tile.highlighted ? 1.015 : 1
            radius: Tokens.radius + Tokens.px(2)
            border.width: tile.activeFocus ? Tokens.border * 2 : Tokens.border
            border.color: tile.highlighted ? Tokens.sun : Tokens.lineSoft
            Behavior on scale { NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease } }
            Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
        }

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: Tokens.px(64)
            radius: Tokens.radius + Tokens.px(2)
            gradient: Gradient {
                orientation: Gradient.Vertical
                GradientStop { position: 0.0; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0) }
                GradientStop { position: 1.0; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.88) }
            }
        }

        Badge {
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.margins: Tokens.s2
            text: tile.game.store.toUpperCase()
        }
        Badge {
            visible: tile.game.update_available === true
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.s2
            text: I18n.tr("UPDATE")
            accent: true
        }
        Badge {
            visible: Daemon.isRunning(tile.game.store, tile.game.id)
            anchors.left: parent.left
            anchors.bottom: parent.bottom
            anchors.margins: Tokens.s2
            text: I18n.tr("PLAYING")
            solid: true
        }
        Text {
            visible: !Daemon.isRunning(tile.game.store, tile.game.id) && tile.highlighted
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.margins: Tokens.s3
            text: I18n.tr("VIEW") + "  →"
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fTiny
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
        }

        Rectangle {
            visible: !!tile.progress
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: Tokens.px(4)
            color: Tokens.tint16
            Rectangle {
                height: parent.height
                width: parent.width * ((tile.progress && tile.progress.percent !== null && tile.progress.percent !== undefined)
                                       ? tile.progress.percent / 100 : 0.04)
                color: Tokens.sun
                Behavior on width { NumberAnimation { duration: Tokens.move } }
            }
        }
    }

    Text {
        id: title
        anchors.top: card.bottom
        anchors.topMargin: Tokens.s3
        width: tile.artW
        text: tile.game.title
        color: tile.highlighted ? Tokens.ink : Tokens.inkDim
        font.family: Tokens.ui
        font.pixelSize: Tokens.fBody
        font.weight: tile.highlighted ? Font.Medium : Font.Normal
        elide: Text.ElideRight
        maximumLineCount: 1
    }
    Text {
        anchors.top: title.bottom
        anchors.topMargin: Tokens.s1
        width: tile.artW
        text: {
            var facts = [tile.game.store.toUpperCase()];
            if (tile.game.favorite === true) facts.unshift("★");
            facts.push(tile.game.installed ? (tile.game.platform || I18n.tr("Installed")) : I18n.tr("Not installed"));
            return facts.join("  ·  ");
        }
        color: Tokens.inkFaint
        font.family: Tokens.ui
        font.pixelSize: Tokens.fMicro
        elide: Text.ElideRight
    }
    Text {
        anchors.bottom: parent.bottom
        width: tile.artW
        visible: text.length > 0
        text: tile.game.installed ? (tile.game.install_size || tile.game.version || "") : ""
        color: Tokens.inkFaint
        font.family: Tokens.ui
        font.pixelSize: Tokens.fTiny
        elide: Text.ElideRight
    }

    TapHandler { onTapped: tile.open() }
    HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
}
