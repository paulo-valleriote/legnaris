package auth_test

import (
	"regexp"
	"strings"
	"testing"

	"legnaris/backend/internal/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// codePattern is the only shape a code may take: six decimal digits, leading
// zeros preserved as string characters (OTP-02).
var codePattern = regexp.MustCompile(`^\d{6}$`)

// TestCodeGenerator_Generate proves the generator never drifts from the
// six-digit shape and reaches a leading-zero value (OTP-02).
func TestCodeGenerator_Generate(t *testing.T) {
	gen := auth.NewCodeGenerator()

	t.Run("Happy path: 10,000 codes are six digits, including a leading zero", func(t *testing.T) {
		leadingZero := false
		for i := 0; i < 10_000; i++ {
			code, err := gen.Generate()
			require.NoError(t, err, "generation must not fail on iteration %d", i)
			require.Truef(t, codePattern.MatchString(code), "iteration %d produced %q", i, code)
			if code[0] == '0' {
				leadingZero = true
			}
		}
		assert.True(t, leadingZero, "10,000 draws must include at least one leading-zero code")
	})
}

// TestCodeDigest_Digest pins the identity-and-purpose binding and the
// determinism of the keyed digest (OTP-03).
func TestCodeDigest_Digest(t *testing.T) {
	digest, err := auth.NewCodeDigest(strings.Repeat("k", 32))
	require.NoError(t, err)

	const (
		userA = "01900000-0000-7000-8000-000000000001"
		userB = "01900000-0000-7000-8000-000000000002"
		code  = "042917"
		other = "042918"
	)

	base := digest.Digest(userA, auth.PurposeVerifyEmail, code)

	t.Run("Happy path: the digest is deterministic", func(t *testing.T) {
		assert.Equal(t, base, digest.Digest(userA, auth.PurposeVerifyEmail, code))
	})

	t.Run("Edge: the account changes the digest", func(t *testing.T) {
		assert.NotEqual(t, base, digest.Digest(userB, auth.PurposeVerifyEmail, code))
	})

	t.Run("Edge: the purpose changes the digest", func(t *testing.T) {
		assert.NotEqual(t, base, digest.Digest(userA, auth.PurposeLoginOTP, code))
	})

	t.Run("Edge: the code changes the digest", func(t *testing.T) {
		assert.NotEqual(t, base, digest.Digest(userA, auth.PurposeVerifyEmail, other))
	})

	t.Run("Edge: a code with a leading zero is digested on its six characters", func(t *testing.T) {
		zeroPadded := digest.Digest(userA, auth.PurposeVerifyEmail, "000042")
		assert.NotEqual(t, digest.Digest(userA, auth.PurposeVerifyEmail, "42"), zeroPadded)
	})
}

// TestCodeDigest_Equal pins the constant-time comparison's observable contract
// (OTP-03).
func TestCodeDigest_Equal(t *testing.T) {
	digest, err := auth.NewCodeDigest(strings.Repeat("k", 32))
	require.NoError(t, err)

	left := digest.Digest("user", auth.PurposeLoginOTP, "123456")
	right := digest.Digest("user", auth.PurposeLoginOTP, "123456")

	t.Run("Happy path: identical digests are equal", func(t *testing.T) {
		assert.True(t, digest.Equal(left, right))
	})

	t.Run("Edge: a one-character difference is rejected", func(t *testing.T) {
		differs := left[:len(left)-1] + "0"
		if differs == left {
			differs = left[:len(left)-1] + "1"
		}
		assert.False(t, digest.Equal(left, differs))
	})

	t.Run("Edge: a different-length value is rejected without panicking", func(t *testing.T) {
		assert.False(t, digest.Equal(left, "short"))
		assert.False(t, digest.Equal("", left))
	})
}

// TestNewCodeDigest pins the secret-strength boundary: absent, blank and 31
// bytes are rejected, 32 bytes are accepted (OTP-18, SC-008).
func TestNewCodeDigest(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{name: "Edge: an empty secret is rejected", secret: "", wantErr: true},
		{name: "Edge: a whitespace-only secret is rejected", secret: strings.Repeat(" ", 32), wantErr: true},
		{name: "Edge: a 31-byte secret is rejected", secret: strings.Repeat("k", 31), wantErr: true},
		{name: "Edge: a 32-byte secret is accepted", secret: strings.Repeat("k", 32), wantErr: false},
		{name: "Happy path: a long secret is accepted", secret: strings.Repeat("k", 64), wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			digest, err := auth.NewCodeDigest(tc.secret)
			if tc.wantErr {
				require.ErrorIs(t, err, auth.ErrCodeSecretWeak)
				assert.Nil(t, digest)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, digest)
		})
	}
}
