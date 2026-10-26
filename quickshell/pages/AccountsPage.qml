import QtQuick
import QtQuick.Controls.Basic
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"

PageBase {
    id: page

    property var flows: ({})
    property var busy: ({})
    property string note: ""
    property bool noteIsError: false

    readonly property var stores: ["epic", "gog", "amazon"]

    function storeName(store) {
        if (store === "epic") return "Epic Games";
        if (store === "gog") return "GOG";
        return "Amazon Games";
    }
    function storeLetter(store) {
        if (store === "epic") return "E";
        if (store === "gog") return "G";
        return "A";
    }

    function setBusy(store, value) {
        var next = Object.assign({}, page.busy);
        if (value) next[store] = true;
        else delete next[store];
        page.busy = next;
    }
    function setFlow(store, value) {
        var next = Object.assign({}, page.flows);
        if (value) next[store] = value;
        else delete next[store];
        page.flows = next;
    }

    function login(store) {
        page.note = "";
        page.noteIsError = false;
        setBusy(store, true);
        Daemon.call("auth.login.auto", { store: store }, (r, e) => {
            setBusy(store, false);
            if (!e) {
                Daemon.refreshAuth();
                page.note = I18n.tr("Signed in to ") + storeName(store);
                return;
            }
            Daemon.call("auth.login", { store: store }, (r2, e2) => {
                if (e2) {
                    page.note = e2.message;
                    page.noteIsError = true;
                    return;
                }
                if (!r2 || r2.state !== "pending") return;
                setFlow(store, { url: r2.url, hint: r2.hint });
                page.note = e.message;
                page.noteIsError = false;
                Qt.openUrlExternally(r2.url);
            });
        });
    }

    function completeLogin(store, code) {
        if (code.length === 0) {
            page.note = I18n.tr("Paste the code from the browser to finish.");
            page.noteIsError = true;
            return;
        }
        Daemon.call("auth.login", { store: store, code: code }, (r, e) => {
            if (e) {
                page.note = e.message;
                page.noteIsError = true;
                return;
            }
            setFlow(store, null);
            page.note = I18n.tr("Signed in to ") + storeName(store);
            page.noteIsError = false;
            Daemon.refreshAuth();
        });
    }

    function cancelLogin(store) {
        setFlow(store, null);
        page.note = "";
    }

    function logout(store) {
        Daemon.call("auth.logout", { store: store }, (r, e) => {
            if (e) {
                page.note = e.message;
                page.noteIsError = true;
                return;
            }
            page.note = I18n.tr("Signed out of ") + storeName(store);
            page.noteIsError = false;
        });
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: Tokens.s5
        spacing: Tokens.s4

        ColumnLayout {
            spacing: Tokens.s1
            Text {
                text: I18n.tr("Accounts")
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fTitle
            }
            Text {
                text: I18n.tr("Sign in once per store. Hikari keeps its own engine sessions, separate from Heroic.")
                color: Tokens.inkFaint
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
        }

        Text {
            visible: page.note.length > 0
            Layout.fillWidth: true
            wrapMode: Text.WordWrap
            text: page.note
            color: page.noteIsError ? Tokens.alert : Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }

        Repeater {
            model: page.stores

            Panel {
                id: row
                required property string modelData
                Layout.fillWidth: true

                readonly property var status: Daemon.authStores[modelData] || ({ logged_in: false })
                readonly property var flow: page.flows[modelData]
                readonly property bool working: page.busy[modelData] === true

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.s3

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: Tokens.s3

                        Rectangle {
                            Layout.preferredWidth: Tokens.px(52)
                            Layout.preferredHeight: Tokens.px(52)
                            radius: Tokens.radius + Tokens.px(2)
                            color: Tokens.tint5
                            border.width: Tokens.border
                            border.color: row.status.logged_in ? Tokens.sun : Tokens.line
                            Text {
                                anchors.centerIn: parent
                                text: page.storeLetter(row.modelData)
                                color: row.status.logged_in ? Tokens.sun : Tokens.inkDim
                                font.family: Tokens.display
                                font.pixelSize: Tokens.fValue
                                font.weight: Font.Medium
                            }
                        }

                        ColumnLayout {
                            spacing: Tokens.s1
                            Text {
                                text: page.storeName(row.modelData)
                                color: Tokens.ink
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fRow
                                font.weight: Font.Medium
                            }
                            RowLayout {
                                spacing: Tokens.s2
                                Badge {
                                    text: row.status.logged_in ? I18n.tr("CONNECTED") : I18n.tr("SIGNED OUT")
                                    accent: row.status.logged_in
                                    muted: !row.status.logged_in
                                }
                                Text {
                                    visible: row.status.logged_in
                                    text: row.status.user || row.status.user_id || ""
                                    color: Tokens.inkDim
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fSmall
                                    elide: Text.ElideRight
                                }
                            }
                        }

                        Item { Layout.fillWidth: true }

                        Text {
                            visible: row.working
                            text: I18n.tr("Waiting for the login window…")
                            color: Tokens.sunDeep
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                        }
                        Btn {
                            visible: !row.status.logged_in && !row.working && !row.flow
                            compact: true
                            primary: true
                            stealFocus: true
                            text: I18n.tr("Log in")
                            onAct: page.login(row.modelData)
                        }
                        Btn {
                            visible: row.status.logged_in
                            compact: true
                            stealFocus: true
                            text: I18n.tr("Log out")
                            onAct: page.logout(row.modelData)
                        }
                    }

                    ColumnLayout {
                        visible: !!row.flow
                        Layout.fillWidth: true
                        spacing: Tokens.s2

                        Text {
                            Layout.fillWidth: true
                            wrapMode: Text.WordWrap
                            text: row.flow ? (I18n.tr("Finish in your browser, then paste the code") + " · " + (row.flow.hint || "")) : ""
                            color: Tokens.inkFaint
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                        }
                        RowLayout {
                            Layout.fillWidth: true
                            spacing: Tokens.s2
                            Field {
                                id: codeField
                                Layout.fillWidth: true
                                placeholder: I18n.tr("Paste the authorization code")
                            }
                            Btn {
                                compact: true
                                primary: true
                                stealFocus: true
                                text: I18n.tr("Finish")
                                onAct: page.completeLogin(row.modelData, codeField.text)
                            }
                            Btn {
                                compact: true
                                text: I18n.tr("Open browser")
                                onAct: if (row.flow) Qt.openUrlExternally(row.flow.url)
                            }
                            Btn {
                                compact: true
                                text: I18n.tr("Cancel")
                                onAct: page.cancelLogin(row.modelData)
                            }
                        }
                    }
                }
            }
        }

        Text {
            Layout.topMargin: Tokens.s2
            Layout.fillWidth: true
            wrapMode: Text.WordWrap
            text: I18n.tr("Automatic login uses a private Chromium window; it reads only the authorization code in the final URL, never your password or cookies. Without Chromium, Hikari opens your default browser and asks for the code.")
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }
        Item { Layout.fillHeight: true }
    }
}
