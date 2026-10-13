package api

import (
	"bytes"
	"encoding/json"
)

var (
	jsonNull   = json.RawMessage(`null`)
	jsonObject = json.RawMessage(`{}`)
)

func normalizeJSON(d json.RawMessage) json.RawMessage {
	if len(d) > 0 {
		buf := new(bytes.Buffer)
		if err := json.Compact(buf, d); err != nil {
			return d
		}
		d = buf.Bytes()

		if bytes.Equal(d, jsonNull) || bytes.Equal(d, jsonObject) {
			return nil
		}
	}
	if len(d) == 0 {
		return nil
	}
	return d
}
