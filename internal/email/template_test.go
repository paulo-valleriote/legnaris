package email_test

import (
	"strings"
	"testing"

	"legnaris/internal/email"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRenderer(t *testing.T) email.Renderer {
	t.Helper()

	renderer, err := email.NewRenderer()
	require.NoError(t, err)
	return renderer
}

// TestRenderer_PerPurpose proves every known purpose renders a code-free
// subject and both bodies carrying the code and its lifetime (EML-11, EML-13).
func TestRenderer_PerPurpose(t *testing.T) {
	renderer := newRenderer(t)
	const code = "042917"
	data := email.Data{Code: code, ExpiresInMinutes: 10}

	subjects := make(map[email.Purpose]string)
	for _, purpose := range []email.Purpose{email.PurposeVerifyEmail, email.PurposeLoginOTP} {
		rendered, err := renderer.Render(purpose, data)
		require.NoError(t, err)
		subjects[purpose] = rendered.Subject

		t.Run(string(purpose)+": both bodies carry the code and its lifetime", func(t *testing.T) {
			assert.Contains(t, rendered.Text, code)
			assert.Contains(t, rendered.HTML, code)
			assert.Contains(t, rendered.Text, "expires in 10 minutes")
			assert.Contains(t, rendered.HTML, "expires in 10 minutes")
		})

		t.Run(string(purpose)+": the subject never carries the code", func(t *testing.T) {
			assert.NotEmpty(t, rendered.Subject)
			assert.NotContains(t, rendered.Subject, code)
		})
	}

	t.Run("Edge: each purpose renders its own subject", func(t *testing.T) {
		assert.NotEqual(t, subjects[email.PurposeVerifyEmail], subjects[email.PurposeLoginOTP])
	})
}

// TestRenderer_UnknownPurpose proves an unknown purpose is a permanent failure
// with no rendered output (EML-11).
func TestRenderer_UnknownPurpose(t *testing.T) {
	renderer := newRenderer(t)

	tests := []struct {
		name    string
		purpose email.Purpose
	}{
		{name: "Error: empty purpose", purpose: email.Purpose("")},
		{name: "Error: an unrelated purpose", purpose: email.Purpose("reset_password")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered, err := renderer.Render(tc.purpose, email.Data{Code: "042917", ExpiresInMinutes: 10})

			require.Error(t, err)
			assert.ErrorIs(t, err, email.ErrUnknownTemplate)
			assert.Equal(t, email.Rendered{}, rendered, "an unknown purpose must render nothing")
		})
	}
}

// TestRenderer_HTMLEscaping proves HTML-significant characters are escaped in
// the HTML body and left verbatim in text: the code is numeric today, but the
// escaping is the contract (EML-11).
func TestRenderer_HTMLEscaping(t *testing.T) {
	renderer := newRenderer(t)
	data := email.Data{Code: "<b>&", ExpiresInMinutes: 5}

	rendered, err := renderer.Render(email.PurposeLoginOTP, data)
	require.NoError(t, err)

	t.Run("Happy path: the text body is verbatim", func(t *testing.T) {
		assert.Contains(t, rendered.Text, "<b>&")
	})

	t.Run("Edge: the HTML body is escaped", func(t *testing.T) {
		assert.Contains(t, rendered.HTML, "&lt;b&gt;&amp;")
		assert.False(t, strings.Contains(rendered.HTML, "<b>&"), "the raw code must not appear in HTML")
	})
}

// TestRenderer_Deterministic proves rendering is pure: the same inputs produce
// the same three strings (EML-11).
func TestRenderer_Deterministic(t *testing.T) {
	renderer := newRenderer(t)
	data := email.Data{Code: "042917", ExpiresInMinutes: 10}

	first, err := renderer.Render(email.PurposeVerifyEmail, data)
	require.NoError(t, err)
	second, err := renderer.Render(email.PurposeVerifyEmail, data)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

// TestRenderer_LeadingZero proves a leading-zero code is rendered verbatim
// rather than coerced to a number (EML-11).
func TestRenderer_LeadingZero(t *testing.T) {
	renderer := newRenderer(t)

	rendered, err := renderer.Render(email.PurposeLoginOTP, email.Data{Code: "042917", ExpiresInMinutes: 10})
	require.NoError(t, err)
	assert.Contains(t, rendered.Text, "042917")
	assert.Contains(t, rendered.HTML, "042917")
}
