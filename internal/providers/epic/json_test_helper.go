package epic

import "encoding/json"

func jsonDecode(b []byte, v any) error { return json.Unmarshal(b, v) }
