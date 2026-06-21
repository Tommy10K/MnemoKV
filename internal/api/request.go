package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const maxJSONBodyBytes int64 = 1 << 20

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return normalizeJSONError(err)
	}

	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("request body must contain exactly one JSON value")
		}
		return normalizeJSONError(err)
	}
	return nil
}

func normalizeJSONError(err error) error {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return fmt.Errorf("request body exceeds %d bytes", maxBytesErr.Limit)
	}
	return err
}
