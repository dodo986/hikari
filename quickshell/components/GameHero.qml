import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../components"

Rectangle {
    required property var page
    id: hero
    Layout.fillWidth: true
    Layout.preferredHeight: Tokens.px(330)
    radius: Tokens.radius + Tokens.px(4)
    color: Tokens.paperLift
    border.width: Tokens.border
    border.color: Tokens.lineSoft
    clip: true

    GameArt {
        id: heroArt
        anchors.fill: parent
        game: page.info
        role: "cover"
        showTitle: false
        artOpacity: 0.88
        scale: 1.0
        Component.onCompleted: heroZoom.start()
    }
    NumberAnimation {
        id: heroZoom
        target: heroArt
        property: "scale"
        from: 1.045
        to: 1.0
        duration: Tokens.durLarge
        easing.type: Easing.Bezier
        easing.bezierCurve: Tokens.curveDefaultEffects
    }
    Rectangle {
        anchors.fill: parent
        gradient: Gradient {
            orientation: Gradient.Horizontal
            GradientStop { position: 0.0; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.96) }
            GradientStop { position: 0.58; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.58) }
            GradientStop { position: 1.0; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.12) }
        }
    }
    Rectangle {
        anchors.fill: parent
        gradient: Gradient {
            orientation: Gradient.Vertical
            GradientStop { position: 0.0; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.05) }
            GradientStop { position: 0.58; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.18) }
            GradientStop { position: 1.0; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.94) }
        }
    }

    GameArt {
        id: portrait
        visible: hero.width > Tokens.px(720)
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: Tokens.s5
        width: Tokens.px(132)
        height: Tokens.px(176)
        game: page.info
        role: "square"
        showTitle: false
        radius: Tokens.radius + Tokens.px(2)
        border.width: Tokens.border
        border.color: Tokens.lineStrong
    }

    ColumnLayout {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: Tokens.s5
        anchors.rightMargin: portrait.visible ? Tokens.px(180) : Tokens.s5
        spacing: Tokens.s3

        RowLayout {
            spacing: Tokens.s2
            Badge { text: page.info ? page.info.store.toUpperCase() : "" }
            Badge {
                visible: page.launching
                text: I18n.tr("LAUNCHING")
                accent: true
            }
            Badge {
                visible: page.running && !page.launching
                text: I18n.tr("PLAYING")
                solid: true
            }
            Badge {
                visible: page.crashed
                text: I18n.tr("CRASHED")
                danger: true
            }
            Badge {
                visible: !page.running && !page.crashed && !page.launching
                text: page.installed ? I18n.tr("INSTALLED") : I18n.tr("NOT INSTALLED")
                muted: !page.installed
            }
            Badge {
                visible: page.hasUpdate
                text: I18n.tr("UPDATE")
                accent: true
            }
        }
        Text {
            Layout.fillWidth: true
            text: page.info ? page.info.title : ""
            color: Tokens.ink
            font.family: Tokens.display
            font.pixelSize: Tokens.fHero
            font.weight: Font.Medium
            wrapMode: Text.WordWrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }
        Text {
            visible: text.length > 0
            text: page.info ? (page.info.developer || "") : ""
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            elide: Text.ElideRight
        }
        Flow {
            Layout.fillWidth: true
            spacing: Tokens.s2
            Badge { visible: !!(page.info && page.info.platform); text: page.info ? page.fact(page.info.platform) : "" }
            Badge { visible: !!(page.info && page.info.install_size); text: page.info ? page.fact(page.info.install_size) : "" }
            Badge { visible: !!(page.info && page.info.version); text: page.info ? page.fact(page.info.version) : "" }
            Badge { visible: page.info && page.info.can_run_offline === true; text: I18n.tr("OFFLINE READY") }
            Badge { visible: page.info && page.info.cloud_saves === true; text: I18n.tr("CLOUD SAVES") }
        }
    }
}
