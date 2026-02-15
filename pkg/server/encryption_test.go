package server

import (
	"testing"

	"github.com/jameshartig/raterudder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredentials(t *testing.T) {
	// 32-byte key for AES-256
	testKey := "01234567890123456789012345678901"

	t.Run("Encrypt and Decrypt", func(t *testing.T) {
		srv := &Server{
			encryptionKey: testKey,
		}

		originalCreds := types.Credentials{
			Franklin: &types.FranklinCredentials{
				Username:    "test@example.com",
				MD5Password: "password123",
			},
		}

		// Encrypt
		encrypted, err := srv.encryptCredentials(t.Context(), originalCreds)
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.NotEqual(t, originalCreds, encrypted) // Should be different bytes

		// Decrypt
		decrypted, err := srv.decryptCredentials(t.Context(), encrypted)
		require.NoError(t, err)
		assert.Equal(t, originalCreds, decrypted)
	})

	t.Run("Decryption with Wrong Key Fails", func(t *testing.T) {
		srv1 := &Server{encryptionKey: testKey}
		srv2 := &Server{encryptionKey: "12345678901234567890123456789012"} // Different key

		originalCreds := types.Credentials{
			Franklin: &types.FranklinCredentials{Username: "test@example.com"},
		}

		encrypted, err := srv1.encryptCredentials(t.Context(), originalCreds)
		require.NoError(t, err)

		_, err = srv2.decryptCredentials(t.Context(), encrypted)
		assert.Error(t, err)
	})

	t.Run("Missing Key Fails", func(t *testing.T) {
		srv := &Server{encryptionKey: ""}

		creds := types.Credentials{
			Franklin: &types.FranklinCredentials{Username: "test@example.com"},
		}

		_, err := srv.encryptCredentials(t.Context(), creds)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no encryption key configured")

		// Let's try decrypting random data
		_, err = srv.decryptCredentials(t.Context(), []byte("some-random-data"))
		assert.Error(t, err)
	})

	t.Run("Malformed Ciphertext", func(t *testing.T) {
		srv := &Server{encryptionKey: testKey}

		// Too short
		_, err := srv.decryptCredentials(t.Context(), []byte("short"))
		assert.Error(t, err)

		// Random junk
		junk := make([]byte, 50)
		_, err = srv.decryptCredentials(t.Context(), junk)
		assert.Error(t, err)
	})

	t.Run("Encrypt Empty Credentials", func(t *testing.T) {
		srv := &Server{encryptionKey: testKey}

		creds := types.Credentials{}
		encrypted, err := srv.encryptCredentials(t.Context(), creds)
		require.NoError(t, err)

		decrypted, err := srv.decryptCredentials(t.Context(), encrypted)
		require.NoError(t, err)
		assert.Equal(t, creds, decrypted)
	})

}
