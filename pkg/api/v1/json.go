package api

import (
	"bytes"
	"encoding/json"
	"strconv"
)

var (
	jsonNull    = json.RawMessage(`null`)
	jsonObject  = json.RawMessage(`{}`)
	doubleQuote = []byte{'"'}
)

func normalizeJSON(d json.RawMessage) json.RawMessage {
	if len(d) > 0 {
		// HACK: right now the ivms101 field is a textarea, so it comes in as a string.
		// This unescapes the string to get the actual JSON.
		if bytes.HasPrefix(d, doubleQuote) && bytes.HasSuffix(d, doubleQuote) {
			if unquoted, err := strconv.Unquote(string(d)); err == nil {
				d = json.RawMessage(unquoted)
			}
		}

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
