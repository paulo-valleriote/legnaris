package json

import (
	encodingJson "encoding/json"
	"errors"
	"io"
	"legnaris/shared/apperr"
	"net/http"

	"github.com/labstack/echo/v5"
)

const MaxBodySize = 1 << 20 // 1MiB

func Decode[T any](c *echo.Context, dst *T) error {
	r := c.Request()

	r.Body = http.MaxBytesReader(c.Response(), r.Body, MaxBodySize)
	defer r.Body.Close()

	dec := encodingJson.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return apperr.NewError(
			apperr.CodeInvalidJSON,
			"request body contains invalid JSON",
			err,
		)
	}

	if dec.Decode(&struct{}{}) != io.EOF {
		return apperr.NewError(
			apperr.CodeUnknownField,
			"request body contains an unknown field",
			errors.New("uknown field inside request body"),
		)
	}

	return nil
}
