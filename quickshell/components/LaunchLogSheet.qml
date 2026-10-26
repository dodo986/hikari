import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../components"

Sheet {
    required property var page
    id: logSheet
    shown: page.logSheetOpen
    title: I18n.tr("Launch log")
    subtitle: I18n.tr("The last launch of this game, its output and the reason it stopped.")
    onCloseRequested: page.logSheetOpen = false

    RowLayout {
        Layout.fillWidth: true
        spacing: Tokens.s3
        Text {
            Layout.fillWidth: true
            text: {
                var rec = page.lastLog;
                if (!rec) return I18n.tr("No launch recorded yet.");
                var bits = [rec.running ? I18n.tr("running") : (rec.state || I18n.tr("ended"))];
                if (rec.exit_code !== undefined && rec.exit_code !== 0)
                    bits.push(I18n.tr("exit") + " " + rec.exit_code);
                return bits.join("  ·  ");
            }
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }
        ActionButton {
            compact: true
            text: I18n.tr("Refresh")
            onAct: page.loadLastLog()
        }
        ActionButton {
            compact: true
            text: I18n.tr("Copy")
            onAct: { logText.copyAll(); page.actionNote = I18n.tr("Copied"); }
        }
    }
    Text {
        Layout.fillWidth: true
        visible: page.lastLog !== null && page.lastLog.error !== undefined && page.lastLog.error !== ""
        text: page.lastLog && page.lastLog.error ? page.lastLog.error : ""
        color: Tokens.alert
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
        wrapMode: Text.WordWrap
    }
    ColumnLayout {
        Layout.fillWidth: true
        visible: page.lastLog !== null && page.lastLog.mod_stop !== undefined && page.lastLog.mod_stop !== null
        spacing: Tokens.s1
        Text {
            Layout.fillWidth: true
            text: page.lastLog && page.lastLog.mod_stop ? page.lastLog.mod_stop.message : ""
            color: Tokens.sunDeep
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WordWrap
        }
        Text {
            Layout.fillWidth: true
            visible: page.lastLog && page.lastLog.mod_stop && page.lastLog.mod_stop.fix !== undefined
            text: page.lastLog && page.lastLog.mod_stop ? (page.lastLog.mod_stop.fix || "") : ""
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WordWrap
        }
        Text {
            Layout.fillWidth: true
            visible: page.lastLog && page.lastLog.mod_stop && page.lastLog.mod_stop.log !== undefined
            text: {
                var note = page.lastLog ? page.lastLog.mod_stop : null;
                if (!note || !note.log) return "";
                var parts = note.log.split("/");
                return I18n.tr("from") + " " + (note.loader || "") + ": " + parts[parts.length - 1];
            }
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            elide: Text.ElideMiddle
        }
    }
    LogView {
        id: logText
        preferredHeight: Tokens.px(340)
        text: {
            var rec = page.lastLog;
            if (!rec || !rec.lines || rec.lines.length === 0)
                return I18n.tr("No output was recorded. The game may have failed before it started; see the command the launcher built under Launch command.");
            return rec.lines.join("\n");
        }
    }
}
