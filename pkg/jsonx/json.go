package jsonx

import "encoding/json"

func MarshalToString(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func MustMarshalToString(v any) string {
	jsn, err := MarshalToString(v)
	if err != nil {
		panic(err)
	}
	return jsn
}
