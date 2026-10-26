import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../components"

Entrance {
    required property var page
    index: 2
    Layout.fillWidth: true
    Flow {
        width: parent.width
        Layout.fillWidth: true
        spacing: Tokens.s4
    Panel {
        width: page.detailPanelWidth
        title: I18n.tr("Game details")
        subtitle: I18n.tr("The daemon's current record for this title.")
        DetailRow { label: I18n.tr("Store"); value: page.info ? page.info.store.toUpperCase() : "—" }
        DetailRow { label: I18n.tr("Platform"); value: page.info ? page.fact(page.info.platform) : "—" }
        DetailRow { label: I18n.tr("Runner"); value: page.info ? page.fact(page.info.runner) : "—" }
        DetailRow { label: I18n.tr("Version"); value: page.info ? page.fact(page.info.version) : "—" }
        DetailRow { label: I18n.tr("Install size"); value: page.info ? page.fact(page.info.install_size) : "—" }
        DetailRow { label: I18n.tr("Install path"); value: page.info ? page.fact(page.info.install_path) : "—" }
        DetailRow { label: I18n.tr("Executable"); value: page.info ? page.fact(page.info.executable) : "—" }
        DetailRow { label: I18n.tr("Source"); value: page.info && page.info.source === "heroic-import" ? I18n.tr("Heroic import") : I18n.tr("Hikari") }
        DetailRow { label: I18n.tr("Offline play"); value: page.info && page.info.can_run_offline === true ? I18n.tr("Available") : I18n.tr("Not reported") }
        DetailRow { label: I18n.tr("Cloud saves"); value: page.info && page.info.cloud_saves === true ? I18n.tr("Supported") : I18n.tr("Not reported") }
    }
}

component DetailRow: RowLayout {
    property string label: ""
    property string value: ""
    Layout.fillWidth: true
    spacing: Tokens.s3
    Text {
        Layout.preferredWidth: Tokens.px(104)
        text: label
        color: Tokens.inkFaint
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
    }
    Text {
        Layout.fillWidth: true
        text: value
        color: Tokens.inkDim
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
        wrapMode: Text.WrapAnywhere
    }
}
}
