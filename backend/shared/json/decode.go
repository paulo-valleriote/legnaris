package json

import (
	"bytes"
	encodingJson "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v5"
)

const MaxBodySize = 1 << 20 // 1MiB

// ErrInvalidJSON, ErrUnknownField and ErrBodyTooLarge are typed decoding errors. This package
// must not import backend/shared/apperr: controllers translate these errors to apperr
// codes at the HTTP boundary.
var (
	ErrInvalidJSON  = errors.New("request body contains invalid JSON")
	ErrUnknownField = errors.New("request body contains an unknown field")
	ErrBodyTooLarge = errors.New("body exceeds the maximum size")
)

func Decode[T any](c *echo.Context, dst *T) error {
	r := c.Request()

	body := http.MaxBytesReader(c.Response(), r.Body, MaxBodySize)
	r.Body = body
	defer func() { _ = body.Close() }()

	dec := encodingJson.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}

	if dec.Decode(&struct{}{}) != io.EOF {
		return ErrUnknownField
	}

	return nil
}

// DecodeBytes decodes one JSON value from a byte slice bounded by MaxBodySize.
// Unlike Decode it tolerates unknown fields: a consumer must not reject a
// message a newer producer wrote during a rolling deploy (WKR-04).
func DecodeBytes[T any](body []byte, dst *T) error {
	if len(body) > MaxBodySize {
		return ErrBodyTooLarge
	}

	dec := encodingJson.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}

	if dec.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("%w: trailing data", ErrInvalidJSON)
	}
	return nil
}
