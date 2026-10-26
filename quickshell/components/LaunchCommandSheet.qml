import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../components"

Sheet {
    required property var page
    id: commandSheet
    shown: page.commandSheetOpen
    title: I18n.tr("Launch command")
    subtitle: I18n.tr("Exactly what the daemon would execute, including its environment diff.")
    onCloseRequested: page.commandSheetOpen = false

    Text {
        Layout.fillWidth: true
        text: page.launchPlan === null
            ? I18n.tr("Loading…")
            : commandText.text
        visible: false
    }
    TextEdit {
        id: commandText
        Layout.fillWidth: true
        readOnly: true
        selectByMouse: true
        wrapMode: TextEdit.WrapAnywhere
        color: Tokens.inkDim
        font.family: Tokens.mono
        font.pixelSize: Tokens.fSmall - 1
        text: {
            var plan = page.launchPlan;
            if (plan === null) return "";
            if (plan.error) return plan.error;
            var lines = [];
            for (var k in plan.env) lines.push(k + "=" + plan.env[k]);
            lines.push("");
            lines.push(plan.argv.join(" "));
            return lines.join("\n");
        }
    }
}
