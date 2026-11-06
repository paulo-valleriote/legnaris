package dice_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/dice"
)

const vectorsPath = "../../../testdata/dice/vectors.json"

type vectorSet struct {
	Version  int               `json:"version"`
	Limits   vectorLimits      `json:"limits"`
	Fields   map[string]string `json:"fields"`
	Valid    []validVector     `json:"valid"`
	Invalid  []invalidVector   `json:"invalid"`
	Fit      []fitVector       `json:"fit"`
	Sections []sectionVector   `json:"sections"`
	Evaluate []evaluateVector  `json:"evaluate"`
}

type vectorLimits struct {
	MaxLength      int `json:"maxLength"`
	MaxTerms       int `json:"maxTerms"`
	MinDice        int `json:"minDice"`
	MaxDice        int `json:"maxDice"`
	MinFaces       int `json:"minFaces"`
	MaxFaces       int `json:"maxFaces"`
	MaxWholeNumber int `json:"maxWholeNumber"`
}

type validVector struct {
	Name      string `json:"name"`
	Expr      string `json:"expr"`
	Canonical string `json:"canonical"`
}

type invalidVector struct {
	Name  string `json:"name"`
	Expr  string `json:"expr"`
	Class string `json:"class"`
}

type fitVector struct {
	Name  string `json:"name"`
	Expr  string `json:"expr"`
	Row   string `json:"row"`
	Fits  bool   `json:"fits"`
	Class string `json:"class"`
}

type sectionVector struct {
	Name  string   `json:"name"`
	Expr  string   `json:"expr"`
	Rows  []string `json:"rows"`
	Fits  bool     `json:"fits"`
	Class string   `json:"class"`
}

type partVector struct {
	Sign  int     `json:"sign"`
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Rolls []int   `json:"rolls"`
}

type evaluateVector struct {
	Name   string             `json:"name"`
	Expr   string             `json:"expr"`
	Row    string             `json:"row"`
	Values map[string]float64 `json:"values"`
	Dice   []int              `json:"dice"`
	Total  float64            `json:"total"`
	Parts  []partVector       `json:"parts"`
	Error  string             `json:"error"`
}

func loadVectors(t *testing.T) vectorSet {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(vectorsPath))
	require.NoError(t, err)
	var set vectorSet
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&set), "every key in the vectors file must be read by a test")
	require.Equal(t, 1, set.Version)
	return set
}

func classOf(t *testing.T, err error) string {
	t.Helper()
	class, ok := dice.ClassOf(err)
	require.True(t, ok, "error %v carries no dice class", err)
	return string(class)
}
