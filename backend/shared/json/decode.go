package json

import (
	encodingJson "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v5"
)

const MaxBodySize = 1 << 20 // 1MiB

// ErrInvalidJSON and ErrUnknownField are typed decoding errors. This package
// must not import backend/shared/apperr: controllers translate these errors to apperr
// codes at the HTTP boundary.
var (
	ErrInvalidJSON  = errors.New("request body contains invalid JSON")
	ErrUnknownField = errors.New("request body contains an unknown field")
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
