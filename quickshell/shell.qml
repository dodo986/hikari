import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

ShellRoot {
    FloatingWindow {
        id: win
        title: I18n.tr("Hikari")
        readonly property int fitW: (win.screen && win.screen.width > 0) ? Math.round(win.screen.width * 0.95) : 1280
        readonly property int fitH: (win.screen && win.screen.height > 0) ? Math.round((win.screen.height - 56) * 0.95) : 840
        minimumSize: Qt.size(Math.min(1024, win.fitW), Math.min(680, win.fitH))
        maximumSize: Qt.size(win.fitW, win.fitH)
        color: Tokens.paper

        Binding {
            target: Tokens
            property: "uiScale"
            value: Tokens.uiScaleFor(win.screen && win.screen.name ? win.screen.name : "")
        }

        onClosed: app.requestQuit()

        Hikari {
            id: app
            anchors.fill: parent
            LayoutMirroring.enabled: I18n.rtl
            LayoutMirroring.childrenInherit: true
        }

        IpcHandler {
            target: "nav"
            function open(page: string): void { app.navigate(page); }
            function page(): string { return app.page; }
            function game(store: string, id: string): void { app.openGame(store, id); }
            function mods(game: string): void { app.navigate("mods"); app.openModGame(game); }
            function modsTab(game: string, tab: string): void {
                app.navigate("mods");
                app.modTab = tab;
                app.openModGame(game);
            }
        }
        IpcHandler {
            target: "hikari"
            function close(): void { app.requestQuit(); }
        }
    }
}
