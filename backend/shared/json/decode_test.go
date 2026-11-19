package json_test

import (
	"encoding/json"
	"strings"
	"testing"

	sharedjson "legnaris/backend/shared/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sample struct {
	Name string `json:"name"`
}

func TestDecodeBytes(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		want    sample
		wantErr error
	}{
		{name: "Happy path: a valid body", body: []byte(`{"name":"a"}`), want: sample{Name: "a"}},
		{name: "Edge: unknown fields are tolerated", body: []byte(`{"name":"a","extra":1}`), want: sample{Name: "a"}},
		{name: "Error: nil body", body: nil, wantErr: sharedjson.ErrInvalidJSON},
		{name: "Error: empty body", body: []byte{}, wantErr: sharedjson.ErrInvalidJSON},
		{name: "Error: malformed JSON", body: []byte(`{"name":`), wantErr: sharedjson.ErrInvalidJSON},
		{name: "Error: wrong field type", body: []byte(`{"name":1}`), wantErr: sharedjson.ErrInvalidJSON},
		{name: "Error: trailing data", body: []byte(`{"name":"a"}{"name":"b"}`), wantErr: sharedjson.ErrInvalidJSON},
		{name: "Edge: the body at the bound decodes", body: []byte(`{"name":"` + strings.Repeat("a", sharedjson.MaxBodySize-len(`{"name":""}`)) + `"}`), want: sample{Name: strings.Repeat("a", sharedjson.MaxBodySize-len(`{"name":""}`))}},
		{name: "Error: the body over the bound", body: []byte(`{"name":"` + strings.Repeat("a", sharedjson.MaxBodySize) + `"}`), wantErr: sharedjson.ErrBodyTooLarge},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got sample
			err := sharedjson.DecodeBytes(tc.body, &got)

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr, "got %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDecodeBytesUseNumber(t *testing.T) {
	tests := []struct {
		name       string
		body       []byte
		wantNumber json.Number
		wantErr    error
	}{
		{name: "Happy path: a number is kept as json.Number", body: []byte(`{"n":1e400}`), wantNumber: "1e400"},
		{name: "Edge: an integer beyond float64 precision keeps its digits", body: []byte(`{"n":9007199254740993}`), wantNumber: "9007199254740993"},
		{name: "Error: trailing data is reported as ErrTrailingData", body: []byte(`{"n":1} {"n":2}`), wantErr: sharedjson.ErrTrailingData},
		{name: "Error: the body over the bound is ErrBodyTooLarge", body: []byte(`{"n":"` + strings.Repeat("a", sharedjson.MaxBodySize) + `"}`), wantErr: sharedjson.ErrBodyTooLarge},
		{name: "Error: malformed JSON is ErrInvalidJSON", body: []byte(`{"n":`), wantErr: sharedjson.ErrInvalidJSON},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]any
			err := sharedjson.DecodeBytesUseNumber(tc.body, &got)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantNumber, got["n"])
		})
	}
}

func TestDecodeBytes_TrailingDataIsInvalidJSON(t *testing.T) {
	var got sample
	err := sharedjson.DecodeBytes([]byte(`{"name":"a"} x`), &got)
	require.ErrorIs(t, err, sharedjson.ErrInvalidJSON)
	require.ErrorIs(t, err, sharedjson.ErrTrailingData)
}
