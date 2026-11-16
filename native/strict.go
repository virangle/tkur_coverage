// SPDX-License-Identifier: EPL-2.0
package main

import (
	"encoding/json"
	"errors"
	"strings"
)

func fields(raw []byte, required, known []string) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if e := json.Unmarshal(raw, &m); e != nil {
		return nil, e
	}
	if m == nil {
		return nil, errors.New("expected JSON object")
	}
	for _, key := range required {
		v, ok := m[key]
		if !ok || string(v) == "null" {
			return nil, errors.New("missing/null required field: " + key)
		}
	}
	for key := range m {
		for _, name := range known {
			if key != name && strings.EqualFold(key, name) {
				return nil, errors.New("ambiguous JSON field case: " + key)
			}
		}
	}
	return m, nil
}
func (v *jsFile) UnmarshalJSON(raw []byte) error {
	names := []string{"path", "statementMap", "fnMap", "branchMap", "s", "f", "b"}
	if _, e := fields(raw, names, names); e != nil {
		return e
	}
	type plain jsFile
	return json.Unmarshal(raw, (*plain)(v))
}
func (v *netMethod) UnmarshalJSON(raw []byte) error {
	names := []string{"Lines", "Branches"}
	if _, e := fields(raw, names, names); e != nil {
		return e
	}
	type plain netMethod
	return json.Unmarshal(raw, (*plain)(v))
}
func (v *netBranch) UnmarshalJSON(raw []byte) error {
	required := []string{"Line", "Offset", "EndOffset", "Path", "Ordinal", "Hits"}
	known := append(append([]string{}, required...), "IsReachable")
	m, e := fields(raw, required, known)
	if e != nil {
		return e
	}
	if r, ok := m["IsReachable"]; ok && string(r) == "null" {
		return errors.New("null IsReachable")
	}
	type plain netBranch
	return json.Unmarshal(raw, (*plain)(v))
}
func (v *jsBranch) UnmarshalJSON(raw []byte) error {
	if _, e := fields(raw, []string{"locations"}, []string{"locations", "type"}); e != nil {
		return e
	}
	type plain jsBranch
	return json.Unmarshal(raw, (*plain)(v))
}
func (v *jsFunction) UnmarshalJSON(raw []byte) error {
	if _, e := fields(raw, []string{"loc"}, []string{"loc"}); e != nil {
		return e
	}
	type plain jsFunction
	return json.Unmarshal(raw, (*plain)(v))
}
func (v *jsLoc) UnmarshalJSON(raw []byte) error {
	if _, e := fields(raw, []string{"start", "end"}, []string{"start", "end"}); e != nil {
		return e
	}
	type plain jsLoc
	return json.Unmarshal(raw, (*plain)(v))
}
func (v *jsPos) UnmarshalJSON(raw []byte) error {
	if _, e := fields(raw, nil, []string{"line", "column"}); e != nil {
		return e
	}
	type plain jsPos
	return json.Unmarshal(raw, (*plain)(v))
}
