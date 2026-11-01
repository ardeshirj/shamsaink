package design

import "encoding/json"

func decodeJSON(s string, into map[string]string) { _ = json.Unmarshal([]byte(s), &into) }
