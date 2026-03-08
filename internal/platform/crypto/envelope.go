package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"cloud.google.com/go/auth/credentials"
	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"google.golang.org/api/option"
)

const dekSize = 32

type EnvelopeCiphertext struct {
	CipherText []byte
	DEKCipher  []byte
	Nonce      []byte
	KeyVersion string
}

type Envelope interface {
	Encrypt(ctx context.Context, plaintext []byte) (*EnvelopeCiphertext, error)
	Decrypt(ctx context.Context, ciphertext *EnvelopeCiphertext) ([]byte, error)
}

type KMSEnvelope struct {
	client      *kms.KeyManagementClient
	cryptoKeyID string
}

func NewKMSEnvelope(ctx context.Context, cryptoKeyID string, credentialsFile string) (*KMSEnvelope, error) {
	var (
		client *kms.KeyManagementClient
		err    error
	)
	if credentialsFile != "" {
		creds, detectErr := credentials.DetectDefault(&credentials.DetectOptions{
			CredentialsFile: credentialsFile,
			Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
		})
		if detectErr != nil {
			return nil, detectErr
		}
		client, err = kms.NewKeyManagementClient(ctx, option.WithAuthCredentials(creds))
	} else {
		client, err = kms.NewKeyManagementClient(ctx)
	}
	if err != nil {
		return nil, err
	}
	return &KMSEnvelope{client: client, cryptoKeyID: cryptoKeyID}, nil
}

func (k *KMSEnvelope) Encrypt(ctx context.Context, plaintext []byte) (*EnvelopeCiphertext, error) {
	dek := make([]byte, dekSize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, err
	}

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	encrypted, err := aesGCMEncrypt(dek, nonce, plaintext)
	if err != nil {
		return nil, err
	}

	resp, err := k.client.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:      k.cryptoKeyID,
		Plaintext: dek,
	})
	if err != nil {
		return nil, err
	}

	return &EnvelopeCiphertext{
		CipherText: encrypted,
		DEKCipher:  resp.Ciphertext,
		Nonce:      nonce,
		KeyVersion: resp.Name,
	}, nil
}

func (k *KMSEnvelope) Decrypt(ctx context.Context, ciphertext *EnvelopeCiphertext) ([]byte, error) {
	resp, err := k.client.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:       k.cryptoKeyID,
		Ciphertext: ciphertext.DEKCipher,
	})
	if err != nil {
		return nil, err
	}
	return aesGCMDecrypt(resp.Plaintext, ciphertext.Nonce, ciphertext.CipherText)
}

type StaticEnvelope struct {
	key []byte
}

func NewStaticEnvelope(base64Key string) (*StaticEnvelope, error) {
	decoded, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, err
	}
	if len(decoded) != dekSize {
		return nil, fmt.Errorf("static key must decode to %d bytes", dekSize)
	}
	return &StaticEnvelope{key: decoded}, nil
}

func (s *StaticEnvelope) Encrypt(_ context.Context, plaintext []byte) (*EnvelopeCiphertext, error) {
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	encrypted, err := aesGCMEncrypt(s.key, nonce, plaintext)
	if err != nil {
		return nil, err
	}
	return &EnvelopeCiphertext{
		CipherText: encrypted,
		DEKCipher:  []byte("static"),
		Nonce:      nonce,
		KeyVersion: "static",
	}, nil
}

func (s *StaticEnvelope) Decrypt(_ context.Context, ciphertext *EnvelopeCiphertext) ([]byte, error) {
	return aesGCMDecrypt(s.key, ciphertext.Nonce, ciphertext.CipherText)
}

func aesGCMEncrypt(key, nonce, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, plaintext, nil), nil
}

func aesGCMDecrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, nil)
}
