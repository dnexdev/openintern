package web

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func javascriptJSON(value any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	body := bytes.TrimSuffix(buf.Bytes(), []byte{'\n'})
	return unescapeLineSeparators(body), nil
}

func unescapeLineSeparators(body []byte) []byte {
	out := make([]byte, 0, len(body))
	for i := 0; i < len(body); {
		if i+6 <= len(body) && body[i] == '\\' && body[i+1] == 'u' &&
			(string(body[i+2:i+6]) == "2028" || string(body[i+2:i+6]) == "2029") {
			backslashes := 0
			for j := i - 1; j >= 0 && body[j] == '\\'; j-- {
				backslashes++
			}
			if backslashes%2 == 0 {
				if body[i+5] == '8' {
					out = append(out, 0xe2, 0x80, 0xa8)
				} else {
					out = append(out, 0xe2, 0x80, 0xa9)
				}
				i += 6
				continue
			}
		}
		out = append(out, body[i])
		i++
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := javascriptJSON(value)
	if err != nil {
		http.Error(w, "JSON encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

type errorResponse struct {
	Error string `json:"error"`
}

type maintenanceResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
