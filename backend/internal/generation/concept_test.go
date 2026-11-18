package generation_test

import (
	"strings"
	"testing"

	"legnaris/backend/internal/generation"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeConcept(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "Happy path: plain text is unchanged", input: "A grizzled dwarf smith", want: "A grizzled dwarf smith"},
		{name: "Happy path: accented Portuguese text is preserved", input: "O coração de um dragão não tem ação", want: "O coração de um dragão não tem ação"},
		{name: "Happy path: emoji is preserved", input: "um mago 🧙 com um dragão 🐉", want: "um mago 🧙 com um dragão 🐉"},
		{name: "Edge: zero-width joiners and zero-width spaces are removed", input: "um\u200d ma\u200bgo", want: "um mago"},
		{name: "Edge: bidi control characters are removed", input: "a\u202eb\u2066c", want: "abc"},
		{name: "Edge: a byte order mark is removed", input: "\ufeffum mago", want: "um mago"},
		{name: "Edge: private use characters are removed", input: "um \ue000mago\uf8ff", want: "um mago"},
		{name: "Edge: control characters are removed", input: "um\x00 \x07mago\x0b", want: "um mago"},
		{name: "Edge: a tab becomes a single space", input: "um\tmago", want: "um mago"},
		{name: "Edge: CRLF becomes a single newline", input: "um mago\r\nda terra", want: "um mago\nda terra"},
		{name: "Edge: a lone CR becomes a newline", input: "um mago\rda terra", want: "um mago\nda terra"},
		{name: "Edge: a newline is kept", input: "um mago\nda terra", want: "um mago\nda terra"},
		{name: "Edge: fullwidth letters are NFKC-folded", input: "ｕｍ ｍａｇｏ", want: "um mago"},
		{name: "Edge: a combining accent is NFKC-composed", input: "coraça\u0303o", want: "coração"},
		{name: "Edge: a concept of only invisible characters becomes empty", input: "\u200b\u200d\u202e\ufeff", want: ""},
		{name: "Edge: surrounding whitespace is trimmed", input: "  \t um mago \n ", want: "um mago"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, generation.SanitizeConcept(tc.input))
		})
	}

	t.Run("Happy path: sanitizing twice changes nothing", func(t *testing.T) {
		input := " \ufeffum\tmago\r\ncom 🧙 e ação\u200b "
		once := generation.SanitizeConcept(input)
		assert.Equal(t, once, generation.SanitizeConcept(once))
		assert.Equal(t, "um mago\ncom 🧙 e ação", once)
	})

	t.Run("Edge: an empty concept stays empty", func(t *testing.T) {
		assert.Equal(t, "", generation.SanitizeConcept(""))
		assert.Equal(t, "", generation.SanitizeConcept("   "))
		assert.Equal(t, "", generation.SanitizeConcept(strings.Repeat("\u200b", 50)))
	})
}
