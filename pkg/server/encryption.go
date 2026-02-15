package server

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/jameshartig/raterudder/pkg/log"
	"github.com/jameshartig/raterudder/pkg/types"
)

func (s *Server) decryptCredentials(ctx context.Context, encrypted []byte) (types.Credentials, error) {
	if len(encrypted) == 0 {
		return types.Credentials{}, nil
	}

	if s.encryptionKey == "" {
		log.Ctx(ctx).ErrorContext(ctx, "cannot decrypt credentials: no encryption key configured")
		return types.Credentials{}, errors.New("cannot decrypt credentials: no encryption key configured")
	}

	key := []byte(s.encryptionKey)
	if len(key) != 32 {
		log.Ctx(ctx).ErrorContext(ctx, "invalid encryption key length (must be 32 bytes)", slog.Int("length", len(key)))
		return types.Credentials{}, errors.New("invalid encryption key length (must be 32 bytes)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to create cipher", slog.Any("error", err))
		return types.Credentials{}, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to create gcm", slog.Any("error", err))
		return types.Credentials{}, fmt.Errorf("failed to create gcm: %w", err)
	}

	if len(encrypted) < gcm.NonceSize() {
		log.Ctx(ctx).ErrorContext(ctx, "malformed encrypted credentials", slog.Int("length", len(encrypted)))
		return types.Credentials{}, errors.New("malformed encrypted credentials")
	}

	nonce, ciphertext := encrypted[:gcm.NonceSize()], encrypted[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to decrypt credentials", slog.Any("error", err))
		return types.Credentials{}, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	var creds types.Credentials
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to unmarshal credentials", slog.Any("error", err))
		return types.Credentials{}, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	return creds, nil
}

func (s *Server) encryptCredentials(ctx context.Context, creds types.Credentials) ([]byte, error) {
	if s.encryptionKey == "" {
		log.Ctx(ctx).ErrorContext(ctx, "cannot encrypt credentials: no encryption key configured")
		return nil, errors.New("cannot encrypt credentials: no encryption key configured")
	}

	jsonBytes, err := json.Marshal(creds)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to marshal credentials", slog.Any("error", err))
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	key := []byte(s.encryptionKey)
	if len(key) != 32 {
		log.Ctx(ctx).ErrorContext(ctx, "invalid encryption key length (must be 32 bytes)", slog.Int("length", len(key)))
		return nil, errors.New("invalid encryption key length (must be 32 bytes)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to create cipher", slog.Any("error", err))
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to create gcm", slog.Any("error", err))
		return nil, fmt.Errorf("failed to create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to generate nonce", slog.Any("error", err))
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, jsonBytes, nil)
	return ciphertext, nil
}
