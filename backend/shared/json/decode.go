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

const MaxBodySize = 1 << 20

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

var ErrTrailingData = errors.New("trailing data after the JSON value")

func DecodeBytes[T any](body []byte, dst *T) error {
	return decodeBytes(body, dst, false)
}

func DecodeBytesUseNumber[T any](body []byte, dst *T) error {
	return decodeBytes(body, dst, true)
}

func decodeBytes[T any](body []byte, dst *T, useNumber bool) error {
	if len(body) > MaxBodySize {
		return ErrBodyTooLarge
	}

	dec := encodingJson.NewDecoder(bytes.NewReader(body))
	if useNumber {
		dec.UseNumber()
	}
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}

	if dec.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("%w: %w", ErrInvalidJSON, ErrTrailingData)
	}
	return nil
}
