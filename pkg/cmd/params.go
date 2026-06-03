package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// parsePair parses a single -d argument.
//
//	key=value    -> string value
//	key:=json    -> raw JSON value (numbers, booleans, arrays, objects, null)
//
// Dotted keys (a.b.c) create nested objects.
func parsePair(pair string) (key string, value interface{}, err error) {
	if i := strings.Index(pair, ":="); i >= 0 {
		key = pair[:i]
		var v interface{}
		if err := json.Unmarshal([]byte(pair[i+2:]), &v); err != nil {
			return "", nil, fmt.Errorf("invalid JSON in %q: %w", pair, err)
		}
		return key, v, nil
	}
	if i := strings.Index(pair, "="); i >= 0 {
		return pair[:i], pair[i+1:], nil
	}
	return "", nil, fmt.Errorf("invalid data %q: expected key=value or key:=json", pair)
}

// buildBody turns -d pairs into a nested JSON object. Returns nil when empty.
func buildBody(pairs []string) (map[string]interface{}, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	body := map[string]interface{}{}
	for _, p := range pairs {
		key, value, err := parsePair(p)
		if err != nil {
			return nil, err
		}
		setNested(body, key, value)
	}
	return body, nil
}

// setNested assigns value at a dotted key path within m.
func setNested(m map[string]interface{}, key string, value interface{}) {
	parts := strings.Split(key, ".")
	cur := m
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = value
			return
		}
		next, ok := cur[p].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			cur[p] = next
		}
		cur = next
	}
}

// buildQuery turns -d pairs into URL query values (string values only; :=
// raw-JSON values are encoded as their JSON text).
func buildQuery(pairs []string) (url.Values, error) {
	q := url.Values{}
	for _, p := range pairs {
		key, value, err := parsePair(p)
		if err != nil {
			return nil, err
		}
		switch v := value.(type) {
		case string:
			q.Add(key, v)
		default:
			encoded, _ := json.Marshal(v)
			q.Add(key, string(encoded))
		}
	}
	return q, nil
}
