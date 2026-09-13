package json

import (
	encodingJson "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const MaxBodySize = 1 << 20 // 1MiB

func decodeJSON[T any](w http.ResponseWriter, r *http.Request, dst *T) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodySize)
	defer r.Body.Close()

	dec := encodingJson.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if dec.Decode(&struct{}{}) != io.EOF {
		return errors.New("JSON payload should contain just one object")
	}

	return nil
}
