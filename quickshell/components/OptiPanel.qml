import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../components"

    Panel {
    id: root
    required property var page
        width: parent.width
        title: I18n.tr("OptiScaler")
    subtitle: I18n.tr("Clean-room upscaling and frame generation, installed beside the game's executable.")
    RowLayout {
        Layout.fillWidth: true
        spacing: Tokens.s2
        Badge {
            text: page.opti.status && page.opti.status.installed ? I18n.tr("INSTALLED") : I18n.tr("NOT INSTALLED")
            accent: page.opti.status && page.opti.status.installed
        }
        Badge { visible: !!(page.opti.status && page.opti.status.version); text: page.opti.status ? page.fact(page.opti.status.version) : "" }
        Badge { visible: !!(page.opti.status && page.opti.status.injection_dll); text: page.opti.status ? page.fact(page.opti.status.injection_dll) : "" }
        Item { Layout.fillWidth: true }
        Text {
            visible: page.opti.gpu !== null
            text: page.opti.gpu ? I18n.tr("Detected GPU") + ": " + page.opti.gpu.name : ""
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            elide: Text.ElideRight
            Layout.maximumWidth: Tokens.px(420)
        }
    }
    Text {
        readonly property string options: (page.opti.status && page.opti.status.launch_options) ? page.opti.status.launch_options : ""
        visible: options.length > 0
        Layout.fillWidth: true
        wrapMode: Text.WordWrap
        text: I18n.tr("Set this in Steam's launch options for this game") + ":\n" + options
        color: Tokens.sunDeep
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
    }
    RowLayout {
        spacing: Tokens.s3
        ActionButton {
            compact: true
            primary: true
            visible: !(page.opti.status && page.opti.status.installed)
            text: I18n.tr("Enable OptiScaler")
            onAct: page.optiInstall({})
        }
        ActionButton {
            compact: true
            visible: !!(page.opti.status && page.opti.status.installed)
            text: I18n.tr("Remove")
            onAct: page.optiRemove()
        }
        Btn {
            compact: true
            text: page.optiAdvanced ? I18n.tr("Hide options") : I18n.tr("Options")
            onAct: page.optiAdvanced = !page.optiAdvanced
        }
        Text {
            visible: page.optiPhase.length > 0
            text: page.optiPhase
            color: Tokens.sunDeep
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }
    }
    ColumnLayout {
        visible: page.optiAdvanced
        spacing: Tokens.s3
        Layout.fillWidth: true
        RowLayout {
            spacing: Tokens.s3
            Text { text: I18n.tr("Version"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
            Field { id: optiVersion; Layout.preferredWidth: Tokens.px(160); placeholder: page.opti.latestTag }
        }
        Flow {
            Layout.fillWidth: true
            spacing: Tokens.s2
            Text { text: I18n.tr("Hook"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
            Repeater {
                model: ["dxgi.dll", "winmm.dll", "version.dll", "d3d12.dll"]
                Btn {
                    required property var modelData
                    compact: true
                    primary: page.optiInjection === modelData
                    armed: page.optiInjection !== modelData
                    text: modelData
                    onAct: page.optiInjection = modelData
                }
            }
        }
        Flow {
            Layout.fillWidth: true
            spacing: Tokens.s2
            Text { text: I18n.tr("Upscaler"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
            Repeater {
                model: ["auto", "fsr", "xess", "dlss"]
                Btn {
                    required property string modelData
                    compact: true
                    primary: page.optiUpscaler === modelData
                    armed: page.optiUpscaler !== modelData
                    text: modelData
                    onAct: page.optiUpscaler = modelData
                }
            }
        }
        RowLayout {
            spacing: Tokens.s3
            Btn {
                compact: true
                primary: page.optiFrameGen
                armed: !page.optiFrameGen
                text: I18n.tr("Frame generation")
                onAct: page.optiFrameGen = !page.optiFrameGen
            }
            Text {
                visible: page.optiFrameGen
                text: I18n.tr("input fsrfg (works on every GPU)")
                color: Tokens.inkFaint
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
        }
        ActionButton {
            compact: true
            primary: true
            text: I18n.tr("Install with these options")
            onAct: page.optiInstall({
                tag: optiVersion.text.length > 0 ? optiVersion.text : page.opti.latestTag,
                injection: page.optiInjection,
                upscaler: page.optiUpscaler === "auto" ? "" : page.optiUpscaler,
                framegen: page.optiFrameGen,
                fg_input: page.optiFrameGen ? "fsrfg" : ""
            })
        }
    }
}
