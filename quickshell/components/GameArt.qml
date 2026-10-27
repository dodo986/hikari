import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

// kind: "portrait" is the 2:3 card face, "wide" the banner, "square" a small mark.
// A landscape image only fills a portrait card when it has pixels to spare; a
// small one is shown whole instead of being zoomed into mush.
Rectangle {
    id: root

    property var game: null
    property string kind: "portrait"
    property bool showTitle: false
    property real artOpacity: 1.0

    readonly property var slots: root.game ? {
        portrait: pick("cover"),
        wide: pick("square"),
        square: pick("icon"),
        logo: pick("logo")
    } : ({})

    function pick(slot) {
        if (!root.game) return "";
        var local = root.game.artwork_local || ({});
        var remote = root.game.artwork || ({});
        var value = local[slot] || remote[slot] || "";
        if (value === "") return "";
        return value.indexOf("://") < 0 ? "file://" + value : value;
    }

    readonly property string portraitArt: slots.portrait || ""
    readonly property string wideArt: slots.wide || ""
    readonly property string logoArt: slots.logo || ""
    readonly property bool hasPortrait: portraitArt !== ""
    readonly property bool hasWide: wideArt !== ""

    readonly property bool faceFromWide: !hasPortrait && hasWide
    readonly property bool enoughPixels: wideProbe.sourceSize.width >= root.width
        && wideProbe.sourceSize.height >= root.height
    readonly property bool letterboxed: root.kind === "portrait" && faceFromWide && !enoughPixels
    readonly property bool portraitPlusLogo: root.kind === "wide" && !hasWide && hasPortrait
    readonly property bool ready: root.kind === "square"
        ? (squareCrop.ready || portraitCrop.ready)
        : (hasPortrait ? portraitCrop.ready : (hasWide ? wideCrop.ready : false))

    color: Tokens.paperLift
    clip: true

    Image {
        id: wideProbe
        source: root.wideArt
        visible: false
        asynchronous: true
    }

    Rectangle {
        anchors.fill: parent
        gradient: Gradient {
            orientation: Gradient.Vertical
            GradientStop { position: 0.0; color: Tokens.tint16 }
            GradientStop { position: 1.0; color: Tokens.paperLift }
        }
    }

    HeroCrop {
        id: portraitCrop
        anchors.fill: parent
        visible: root.hasPortrait && root.kind !== "square"
        source: root.portraitArt
        focalY: root.kind === "wide" ? 0.4 : 0.5
        strength: root.artOpacity * (root.portraitPlusLogo ? 0.55 : 1.0)
        opacity: root.hasPortrait && root.kind !== "square" && portraitCrop.ready ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
    }

    HeroCrop {
        id: wideCrop
        anchors.fill: parent
        visible: root.hasWide && (root.kind !== "portrait" || root.faceFromWide)
        source: root.wideArt
        focalY: 0.42
        strength: root.artOpacity * (root.letterboxed ? 0.28 : 1.0)
        opacity: root.hasWide && wideCrop.ready ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
    }

    HeroCrop {
        id: squareCrop
        anchors.fill: parent
        visible: root.kind === "square" && slots.square !== ""
        source: slots.square || ""
        strength: root.artOpacity
        opacity: root.kind === "square" && squareCrop.ready ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
    }

    Image {
        id: banner
        anchors.centerIn: parent
        visible: root.letterboxed && banner.status === Image.Ready
        source: root.wideArt
        asynchronous: true
        fillMode: Image.PreserveAspectFit
        width: parent.width
        height: sourceSize.width > 0 ? Math.min(parent.height * 0.7, width * sourceSize.height / sourceSize.width) : parent.height * 0.4
        smooth: true
    }

    Rectangle {
        visible: (root.letterboxed || root.kind === "wide" || root.showTitle) && root.ready
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: parent.height * (root.letterboxed ? 0.45 : 0.55)
        gradient: Gradient {
            orientation: Gradient.Vertical
            GradientStop { position: 0.0; color: "transparent" }
            GradientStop { position: 1.0; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.8) }
        }
    }

    Image {
        anchors.centerIn: parent
        visible: root.logoArt !== "" && (root.letterboxed || root.portraitPlusLogo)
        source: root.logoArt
        asynchronous: true
        fillMode: Image.PreserveAspectFit
        width: Math.min(parent.width * 0.62, sourceSize.width > 0 ? sourceSize.width : parent.width * 0.62)
        height: Math.min(parent.height * 0.34, sourceSize.height > 0 ? sourceSize.height : parent.height * 0.34)
        smooth: true
    }

    Text {
        visible: root.showTitle && root.kind === "portrait" && root.letterboxed && root.logoArt === ""
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
        visible: root.showTitle && !!root.game && !!root.game.title && !root.hasPortrait
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
