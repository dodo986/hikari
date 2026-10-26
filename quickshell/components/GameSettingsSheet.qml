import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../components"

Sheet {
    required property var page
    id: settingsSheet
    shown: page.settingsSheetOpen
    title: I18n.tr("Game settings")
    subtitle: I18n.tr("Changes apply immediately. The prefix and save path apply when you save.")
    onCloseRequested: page.settingsSheetOpen = false

    function syncFields() {
        if (!page.settings) return;
        prefixField.text = page.settings.wine_prefix || "";
        savesField.text = page.settings.saves_path || "";
    }

    function takeDraft() {
        var next = Object.assign({}, page.draft || {});
        next.wine_prefix = prefixField.text.trim();
        next.saves_path = savesField.text.trim();
        return next;
    }

    Text {
        text: I18n.tr("LAUNCH OPTIONS")
        color: Tokens.inkFaint
        font.family: Tokens.ui
        font.pixelSize: Tokens.fTiny
        font.letterSpacing: Tokens.trackMark
    }
    Text {
        visible: page.launchOptions.length === 0
        text: I18n.tr("This game reports no extra launch options.")
        color: Tokens.inkFaint
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
    }
    Flow {
        Layout.fillWidth: true
        spacing: Tokens.s2
        Repeater {
            model: page.launchOptions
            ActionButton {
                required property var modelData
                required property int index
                compact: true
                primary: page.launchOptionIndex === index
                text: modelData.name
                onAct: page.launchOptionIndex = index
            }
        }
    }

    Rectangle { Layout.fillWidth: true; height: 1; color: Tokens.lineSoft }

    Text {
        text: I18n.tr("COMPATIBILITY")
        color: Tokens.inkFaint
        font.family: Tokens.ui
        font.pixelSize: Tokens.fTiny
        font.letterSpacing: Tokens.trackMark
    }
    Text {
        Layout.fillWidth: true
        text: {
            if (!page.settings || !page.settings.wine_version)
                return I18n.tr("system wine");
            var v = page.settings.wine_version;
            var name = v.name && v.name.length > 0 ? v.name : I18n.tr("system wine");
            return name + "  ·  " + (v.type || "wine");
        }
        color: Tokens.ink
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
        elide: Text.ElideRight
    }
    RowLayout {
        Layout.fillWidth: true
        spacing: Tokens.s3
        ActionButton {
            compact: true
            text: page.winePickerOpen ? I18n.tr("Hide versions") : I18n.tr("Change version")
            onAct: page.winePickerOpen = !page.winePickerOpen
        }
        ActionButton {
            compact: true
            text: I18n.tr("Get latest Proton")
            onAct: page.downloadLatestProton()
        }
    }
    Flow {
        Layout.fillWidth: true
        visible: page.winePickerOpen
        spacing: Tokens.s2
        Repeater {
            model: page.wineVersions
            ActionButton {
                required property var modelData
                compact: true
                primary: page.draft && page.draft.wine_version &&
                         page.draft.wine_version.bin === modelData.bin
                text: modelData.name + " (" + modelData.type + ")"
                onAct: page.chooseWine(modelData)
            }
        }
    }
    RowLayout {
        Layout.fillWidth: true
        spacing: Tokens.s3
        Text {
            text: I18n.tr("Wine prefix")
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }
        Field {
            id: prefixField
            Layout.fillWidth: true
            placeholder: I18n.tr("one prefix per game")
        }
    }

    Rectangle { Layout.fillWidth: true; height: 1; color: Tokens.lineSoft }

    Text {
        text: I18n.tr("SAVES")
        color: Tokens.inkFaint
        font.family: Tokens.ui
        font.pixelSize: Tokens.fTiny
        font.letterSpacing: Tokens.trackMark
    }
    RowLayout {
        Layout.fillWidth: true
        spacing: Tokens.s3
        Text {
            text: I18n.tr("Save path")
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }
        Field {
            id: savesField
            Layout.fillWidth: true
            placeholder: I18n.tr("the game's save directory")
        }
        ActionButton {
            compact: true
            visible: page.epicGame
            text: I18n.tr("Download")
            onAct: page.syncSaves("download")
        }
        ActionButton {
            compact: true
            visible: page.epicGame
            text: I18n.tr("Upload")
            onAct: page.syncSaves("upload")
        }
    }

    Rectangle { Layout.fillWidth: true; height: 1; color: Tokens.lineSoft }

    Text {
        text: I18n.tr("LAUNCH EXTRAS")
        color: Tokens.inkFaint
        font.family: Tokens.ui
        font.pixelSize: Tokens.fTiny
        font.letterSpacing: Tokens.trackMark
    }
    ExtraRow {
        label: I18n.tr("MangoHud")
        hint: I18n.tr("FPS and frame-time overlay")
        checked: page.draft ? page.draft.show_mangohud === true : false
        onToggled: v => page.toggleExtra("show_mangohud", v)
    }
    ExtraRow {
        label: I18n.tr("GameMode")
        hint: I18n.tr("CPU governor and scheduling for the game process")
        checked: page.draft ? page.draft.use_gamemode === true : false
        onToggled: v => page.toggleExtra("use_gamemode", v)
    }
    ExtraRow {
        label: I18n.tr("FSR (Proton)")
        hint: I18n.tr("Full-screen FSR inside Proton")
        available: !page.plainWine
        checked: page.draft ? page.draft.enable_fsr === true : false
        onToggled: v => page.toggleExtra("enable_fsr", v)
    }
    ExtraRow {
        label: I18n.tr("Offline mode")
        hint: I18n.tr("Launch without store checks when the game allows it")
        checked: page.draft ? page.draft.offline_mode === true : false
        onToggled: v => page.toggleExtra("offline_mode", v)
    }
    ExtraRow {
        label: I18n.tr("Disable umu")
        hint: I18n.tr("Use the Proton build directly instead of umu-run")
        checked: page.draft ? page.draft.disable_umu === true : false
        onToggled: v => page.toggleExtra("disable_umu", v)
    }
    ExtraRow {
        label: I18n.tr("DXVK")
        hint: page.plainWine ? I18n.tr("Newest DXVK in the game prefix") : I18n.tr("Plain Wine only")
        available: page.plainWine
        checked: page.draft ? page.draft.auto_install_dxvk === true : false
        onToggled: v => page.toggleExtra("auto_install_dxvk", v)
    }
    ExtraRow {
        label: I18n.tr("VKD3D-Proton")
        hint: page.plainWine ? I18n.tr("Direct3D 12 translation for plain Wine") : I18n.tr("Plain Wine only")
        available: page.plainWine
        checked: page.draft ? page.draft.auto_install_vkd3d === true : false
        onToggled: v => page.toggleExtra("auto_install_vkd3d", v)
    }
    ExtraRow {
        label: I18n.tr("DXVK-NVAPI")
        hint: page.plainWine ? I18n.tr("DLSS hooks for plain Wine") : I18n.tr("Plain Wine only")
        available: page.plainWine
        checked: page.draft ? page.draft.auto_install_dxvk_nvapi === true : false
        onToggled: v => page.toggleExtra("auto_install_dxvk_nvapi", v)
    }
    ExtraRow {
        label: I18n.tr("EAC runtime")
        hint: page.plainWine ? I18n.tr("Proton only") : I18n.tr("Easy Anti-Cheat runtime for Proton")
        available: !page.plainWine
        checked: page.draft ? page.draft.eac_runtime === true : false
        onToggled: v => page.toggleExtra("eac_runtime", v)
    }
    ExtraRow {
        label: I18n.tr("BattlEye runtime")
        hint: page.plainWine ? I18n.tr("Proton only") : I18n.tr("BattlEye runtime for Proton")
        available: !page.plainWine
        checked: page.draft ? page.draft.battleye_runtime === true : false
        onToggled: v => page.toggleExtra("battleye_runtime", v)
    }
    ExtraRow {
        label: I18n.tr("Epic EOS overlay")
        hint: page.epicGame ? I18n.tr("Hooks the game; off by default") : I18n.tr("Epic games only")
        available: page.epicGame
        checked: page.draft ? page.draft.enable_eos_overlay === true : false
        onToggled: v => page.toggleExtra("enable_eos_overlay", v)
    }

    Rectangle { Layout.fillWidth: true; height: 1; color: Tokens.lineSoft }

    RowLayout {
        Layout.fillWidth: true
        spacing: Tokens.s3
        ActionButton {
            compact: true
            text: I18n.tr("Save prefix and save path")
            onAct: page.saveNow(I18n.tr("Settings saved"))
        }
        ActionButton {
            compact: true
            visible: !!(page.installed && page.info && page.info.store !== "local")
            text: page.repairing ? I18n.tr("Repairing…") : I18n.tr("Verify & repair")
            onAct: page.repairGame()
        }
        ActionButton {
            compact: true
            visible: !!(page.info && page.info.store === "local")
            text: I18n.tr("Remove from library")
            onAct: page.removeLocal()
        }
        Item { Layout.fillWidth: true }
        Text {
            text: page.actionNote
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            elide: Text.ElideRight
            Layout.maximumWidth: Tokens.px(260)
        }
    }

component ExtraRow: RowLayout {
    id: extra
    property string label: ""
    property string hint: ""
    property bool available: true
    property bool checked: false
    signal toggled(bool value)

    Layout.fillWidth: true
    spacing: Tokens.s3

    ColumnLayout {
        Layout.fillWidth: true
        spacing: Tokens.s1
        Text {
            text: extra.label
            color: extra.available ? Tokens.ink : Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }
        Text {
            Layout.fillWidth: true
            text: extra.hint
            color: extra.available ? Tokens.inkFaint : Tokens.alert
            font.family: Tokens.ui
            font.pixelSize: Tokens.fTiny
            wrapMode: Text.WordWrap
        }
    }
    Toggle {
        on: extra.checked
        enabled: extra.available
        activeFocusOnTab: extra.available
        opacity: extra.available ? 1 : 0.4
        onToggled: value => extra.toggled(value)
    }
}
}
