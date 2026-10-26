#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
runner="${QMLTESTRUNNER:-/usr/lib/qt6/bin/qmltestrunner}"
if [ ! -x "$runner" ]; then
	runner="$(command -v qmltestrunner6 || command -v qmltestrunner || true)"
fi
if [ -z "$runner" ]; then
	echo "test-ui: no Qt 6 qmltestrunner found (install qt6-declarative)" >&2
	exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/components"

stub='
    QtObject {
        id: tok
        function px(v) { return v }
        property color paper: "#161616"
        property color paperLift: "#1e1e1e"
        property color ink: "#f2f2f2"
        property color inkFaint: "#9a9a9a"
        property color inkDim: "#c8c8c8"
        property color inkMuted: "#777777"
        property color line: "#333333"
        property color lineSoft: "#2a2a2a"
        property color lineStrong: "#444444"
        property color sun: "#f2c14e"
        property color bone: "#e9e3d5"
        property color inkOnBone: "#161616"
        property color alert: "#e5484d"
        property color tint5: "#1a1a1a"
        property color tint10: "#222222"
        property color tint16: "#2a2a2a"
        property int s1: 2
        property int s2: 4
        property int s3: 8
        property int s4: 16
        property int s5: 20
        property int s6: 32
        property int s7: 48
        property int radius: 4
        property int border: 1
        property int fValue: 22
        property int fSmall: 11
        property int fTiny: 10
        property int fMicro: 9
        property int fBody: 12
        property string display: "sans"
        property string ui: "sans"
        property string mono: "monospace"
        property real trackMark: 0.5
        property real trackLabel: 0.5
        property int snap: 80
        property int easeSnap: Easing.OutCubic
    }
    QtObject {
        id: i18n
        function tr(s) { return s }
    }
'

for name in Sheet.qml ActionButton.qml; do
	python3 - "$here/quickshell/components/$name" "$work/components/$name" "$stub" <<'PY'
import sys
src, dst, stub = sys.argv[1], sys.argv[2], sys.argv[3]
s = open(src).read()
s = s.replace('import Ryoku.Ui\n', '').replace('import Ryoku.Ui.Singletons\n', '')
s = s.replace('Tokens.', 'tok.').replace('I18n.tr(', 'i18n.tr(')
i = s.index('{')
open(dst, 'w').write(s[:i + 1] + stub + s[i + 1:])
PY
done

cp "$here/quickshell/tests/tst_sheet.qml" "$work/tst_sheet.qml"
sed -i 's#"../components"#"components"#' "$work/tst_sheet.qml"

cd "$work"
QML_IMPORT_PATH="$work" QT_QPA_PLATFORM=offscreen \
	"$runner" -input tst_sheet.qml -o -,txt
