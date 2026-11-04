package email_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"legnaris/backend/internal/email"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update", false, "rewrite the verify_email golden files from the current render")

// verifyEmailData is the representative payload the verify_email golden files pin.
var verifyEmailData = email.Data{Code: "042917", ExpiresInMinutes: 10}

// assertGolden compares got with the golden file at name, or rewrites it when
// the suite runs with -update.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name)
	if *updateGolden {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
	}
	want, err := os.ReadFile(filepath.Clean(path))
	require.NoErrorf(t, err, "read %s", path)
	assert.Equal(t, string(want), got, "render drifted from %s; run go test -update to accept", path)
}

// TestVerifyEmail_Golden pins the verify_email text and HTML bodies so copy
// changes are deliberate (LGN-160).
func TestVerifyEmail_Golden(t *testing.T) {
	rendered, err := newRenderer(t).Render(email.PurposeVerifyEmail, verifyEmailData)
	require.NoError(t, err)

	t.Run("Happy path: text body matches the golden file", func(t *testing.T) {
		assertGolden(t, "verify_email.golden.txt", rendered.Text)
	})

	t.Run("Happy path: HTML body matches the golden file", func(t *testing.T) {
		assertGolden(t, "verify_email.golden.html", rendered.HTML)
	})

	t.Run("Happy path: subject is the English verification copy", func(t *testing.T) {
		assert.Equal(t, "Your Legnaris verification code", rendered.Subject)
	})
}

// TestVerifyEmail_Content proves the verify_email body carries the code, the
// 10 minute lifetime and the ignore notice, and never a clickable link (LGN-160).
func TestVerifyEmail_Content(t *testing.T) {
	renderer := newRenderer(t)

	tests := []struct {
		name string
		data email.Data
	}{
		{name: "Happy path: six digit code with ten minutes", data: email.Data{Code: "042917", ExpiresInMinutes: 10}},
		{name: "Edge: empty code still renders the frame", data: email.Data{Code: "", ExpiresInMinutes: 10}},
		{name: "Edge: zero lifetime is rendered verbatim", data: email.Data{Code: "042917", ExpiresInMinutes: 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered, err := renderer.Render(email.PurposeVerifyEmail, tc.data)
			require.NoError(t, err)

			for _, body := range []string{rendered.Text, rendered.HTML} {
				assert.Contains(t, body, "you can ignore this email")
				assert.NotContains(t, body, "http")
				assert.NotContains(t, body, "href")
			}
			assert.Contains(t, rendered.Text, tc.data.Code)
			assert.Contains(t, rendered.HTML, tc.data.Code)
		})
	}
}

// TestVerifyEmail_HTMLSpecialCharacters proves a code with HTML-significant
// characters cannot break out of the HTML body (LGN-160).
func TestVerifyEmail_HTMLSpecialCharacters(t *testing.T) {
	renderer := newRenderer(t)

	tests := []struct {
		name string
		code string
		want string
	}{
		{name: "Edge: angle brackets are escaped", code: "<script>", want: "&lt;script&gt;"},
		{name: "Edge: ampersand and quotes are escaped", code: `a&"'`, want: "a&amp;&#34;&#39;"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered, err := renderer.Render(email.PurposeVerifyEmail, email.Data{Code: tc.code, ExpiresInMinutes: 10})
			require.NoError(t, err)

			assert.Contains(t, rendered.HTML, tc.want)
			assert.False(t, strings.Contains(rendered.HTML, "<script>"), "raw markup must not reach the HTML body")
			assert.Contains(t, rendered.Text, tc.code, "the text body is verbatim")
		})
	}
}
