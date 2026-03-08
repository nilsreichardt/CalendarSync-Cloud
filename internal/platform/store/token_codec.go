package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/inovex/CalendarSync/internal/platform/crypto"
	"golang.org/x/oauth2"
)

type TokenCodec struct {
	primary      crypto.Envelope
	legacyStatic crypto.Envelope
}

func NewTokenCodec(envelope crypto.Envelope) *TokenCodec {
	return &TokenCodec{primary: envelope}
}

func NewTokenCodecWithLegacy(primary crypto.Envelope, legacyStatic crypto.Envelope) *TokenCodec {
	return &TokenCodec{
		primary:      primary,
		legacyStatic: legacyStatic,
	}
}

func (c *TokenCodec) EncryptToken(ctx context.Context, connectionID uuid.UUID, provider string, token *oauth2.Token) (*EncryptedToken, error) {
	by, err := json.Marshal(token)
	if err != nil {
		return nil, err
	}
	encrypted, err := c.primary.Encrypt(ctx, by)
	if err != nil {
		return nil, err
	}
	return &EncryptedToken{
		ConnectionID: connectionID,
		Provider:     provider,
		CipherText:   encrypted.CipherText,
		DEKCipher:    encrypted.DEKCipher,
		Nonce:        encrypted.Nonce,
		KeyVersion:   encrypted.KeyVersion,
	}, nil
}

func (c *TokenCodec) DecryptToken(ctx context.Context, encrypted *EncryptedToken) (*oauth2.Token, error) {
	envelope := c.primary
	if encrypted.KeyVersion == "static" && c.legacyStatic != nil {
		envelope = c.legacyStatic
	}
	plaintext, err := envelope.Decrypt(ctx, &crypto.EnvelopeCiphertext{
		CipherText: encrypted.CipherText,
		DEKCipher:  encrypted.DEKCipher,
		Nonce:      encrypted.Nonce,
		KeyVersion: encrypted.KeyVersion,
	})
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(plaintext, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func (c *TokenCodec) ReencryptIfLegacy(ctx context.Context, encrypted *EncryptedToken) (*EncryptedToken, error) {
	if encrypted.KeyVersion != "static" || c.legacyStatic == nil {
		return nil, nil
	}
	token, err := c.DecryptToken(ctx, encrypted)
	if err != nil {
		return nil, err
	}
	return c.EncryptToken(ctx, encrypted.ConnectionID, encrypted.Provider, token)
}
