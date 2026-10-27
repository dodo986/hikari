import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

// kind: "portrait" is the 2:3 card face, "wide" the banner, "square" a small mark.
// The mode is decided from images that actually loaded, so a dead URL falls
// through to whatever the game does have instead of leaving an empty card.
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
    readonly property string squareArt: slots.square || ""

    readonly property bool portraitLoaded: faceProbe.status === Image.Ready
    readonly property bool wideLoaded: wideProbe.status === Image.Ready
    readonly property bool squareLoaded: squareProbe.status === Image.Ready
    readonly property bool portraitTall: portraitLoaded && faceProbe.sourceSize.height > faceProbe.sourceSize.width
    readonly property bool landscapeFace: !portraitTall && (portraitLoaded || wideLoaded)
    readonly property string landscapeArt: !portraitTall && portraitLoaded ? portraitArt : wideArt
    readonly property bool landscapeEnough: landscapeProbe.sourceSize.width >= root.width
        && landscapeProbe.sourceSize.height >= root.height
    // A 3:1 hero band has nothing worth cropping out of it; 16:9 key art does.
    readonly property real landscapeAspect: landscapeProbe.sourceSize.height > 0
        ? landscapeProbe.sourceSize.width / landscapeProbe.sourceSize.height : 0
    readonly property bool landscapeFillable: landscapeEnough && landscapeAspect > 0 && landscapeAspect < 2.6

    readonly property string mode: {
        if (root.kind === "square") {
            return root.squareLoaded || root.portraitLoaded ? "square" : "empty";
        }
        if (root.kind === "wide") {
            if (root.wideLoaded) return "fill";
            if (root.portraitLoaded) return "dimmed";
            return "empty";
        }
        if (root.portraitTall) return "face";
        if (root.landscapeFace) return root.landscapeFillable ? "fill" : "whole";
        return "empty";
    }

    readonly property bool ready: mode !== "empty"
    readonly property bool logoOverlaid: logoArt !== "" && (mode === "whole" || mode === "dimmed")

    color: Tokens.paperLift
    clip: true

    Image {
        id: faceProbe
        source: root.portraitArt
        visible: false
        asynchronous: true
    }

    Image {
        id: wideProbe
        source: root.wideArt
        visible: false
        asynchronous: true
    }

    Image {
        id: landscapeProbe
        source: root.landscapeArt
        visible: false
        asynchronous: true
    }

    Image {
        id: squareProbe
        source: root.squareArt
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
        visible: root.mode === "face" || root.mode === "dimmed" || (root.kind === "square" && !root.squareLoaded)
        source: root.portraitArt
        focalY: root.kind === "wide" ? 0.4 : 0.5
        strength: root.artOpacity * (root.mode === "dimmed" ? 0.55 : 1.0)
        opacity: visible && portraitCrop.ready ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
    }

    HeroCrop {
        id: fillCrop
        anchors.fill: parent
        visible: root.mode === "fill"
        source: root.landscapeArt
        focalY: 0.42
        strength: root.artOpacity
        opacity: visible && fillCrop.ready ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
    }

    HeroCrop {
        id: squareCrop
        anchors.fill: parent
        visible: root.kind === "square" && root.squareLoaded
        source: root.squareArt
        strength: root.artOpacity
        opacity: visible && squareCrop.ready ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
    }

    HeroCrop {
        id: backdropCrop
        anchors.fill: parent
        visible: root.mode === "whole"
        source: root.landscapeArt
        focalY: 0.42
        strength: root.artOpacity * 0.3
        opacity: visible && backdropCrop.ready ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
    }

    Image {
        id: banner
        anchors.centerIn: parent
        visible: root.mode === "whole"
        source: root.landscapeArt
        asynchronous: true
        fillMode: Image.PreserveAspectFit
        width: parent.width
        height: sourceSize.width > 0 ? parent.width * sourceSize.height / sourceSize.width : parent.height * 0.4
        smooth: true
    }

    Rectangle {
        visible: root.ready && root.showTitle
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: parent.height * (root.mode === "whole" ? 0.4 : 0.55)
        gradient: Gradient {
            orientation: Gradient.Vertical
            GradientStop { position: 0.0; color: "transparent" }
            GradientStop { position: 1.0; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.8) }
        }
    }

    Image {
        anchors.centerIn: parent
        visible: root.logoOverlaid && status === Image.Ready
        source: root.logoArt
        asynchronous: true
        fillMode: Image.PreserveAspectFit
        width: Math.min(parent.width * 0.66, sourceSize.width > 0 ? sourceSize.width : parent.width * 0.66)
        height: Math.min(parent.height * 0.34, sourceSize.height > 0 ? sourceSize.height : parent.height * 0.34)
        smooth: true
    }

    Text {
        visible: root.showTitle && root.mode === "whole" && !root.logoOverlaid
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
        visible: root.showTitle && !!root.game && !!root.game.title && root.mode !== "face"
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
