package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/inovex/CalendarSync/internal/platform/crypto"
	"golang.org/x/oauth2"
)

type TokenCodec struct {
	envelope crypto.Envelope
}

func NewTokenCodec(envelope crypto.Envelope) *TokenCodec {
	return &TokenCodec{envelope: envelope}
}

func (c *TokenCodec) EncryptToken(ctx context.Context, connectionID uuid.UUID, provider string, token *oauth2.Token) (*EncryptedToken, error) {
	by, err := json.Marshal(token)
	if err != nil {
		return nil, err
	}
	encrypted, err := c.envelope.Encrypt(ctx, by)
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
	plaintext, err := c.envelope.Decrypt(ctx, &crypto.EnvelopeCiphertext{
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
