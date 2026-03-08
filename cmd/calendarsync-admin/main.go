package main

import (
	"context"
	"fmt"
	"log"
	"os"

	platformcrypto "github.com/inovex/CalendarSync/internal/platform/crypto"
	"github.com/inovex/CalendarSync/internal/platform/store"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: calendarsync-admin reencrypt-tokens")
	}

	switch os.Args[1] {
	case "reencrypt-tokens":
		if err := reencryptTokens(context.Background()); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown command: %s", os.Args[1])
	}
}

func reencryptTokens(ctx context.Context) error {
	st, err := store.NewPostgresStore(mustEnv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer func() {
		_ = st.Close()
	}()

	kmsEnvelope, err := platformcrypto.NewKMSEnvelope(ctx, mustEnv("KMS_CRYPTO_KEY"), os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		return err
	}
	legacyEnvelope, err := platformcrypto.NewStaticEnvelope(mustEnv("CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64"))
	if err != nil {
		return err
	}
	codec := store.NewTokenCodecWithLegacy(kmsEnvelope, legacyEnvelope)

	rows, err := st.DB().QueryContext(ctx, `
		SELECT connection_id, provider, cipher_text, dek_cipher_text, nonce, key_version
		FROM encrypted_oauth_tokens
		WHERE key_version = 'static'
	`)
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()

	count := 0
	for rows.Next() {
		var token store.EncryptedToken
		if err := rows.Scan(&token.ConnectionID, &token.Provider, &token.CipherText, &token.DEKCipher, &token.Nonce, &token.KeyVersion); err != nil {
			return err
		}
		rewritten, err := codec.ReencryptIfLegacy(ctx, &token)
		if err != nil {
			return err
		}
		if rewritten == nil {
			continue
		}
		if err := st.StoreEncryptedToken(ctx, *rewritten); err != nil {
			return err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}

	fmt.Printf("reencrypted %d token(s)\n", count)
	return nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return v
}
