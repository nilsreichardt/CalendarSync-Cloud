package main

import (
	"context"
	"log"
	"os"
	"time"

	platformcrypto "github.com/inovex/CalendarSync/internal/platform/crypto"
	"github.com/inovex/CalendarSync/internal/platform/store"
	"github.com/inovex/CalendarSync/internal/platform/worker"
)

func main() {
	ctx := context.Background()
	st, err := store.NewPostgresStore(mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	tokenCodec, err := newTokenCodec(ctx)
	if err != nil {
		log.Fatal(err)
	}

	runner := worker.NewRunner(st, tokenCodec, mustEnv("GOOGLE_OAUTH_CLIENT_ID"), mustEnv("GOOGLE_OAUTH_CLIENT_SECRET"))

	for {
		worked, err := runner.RunOnce(ctx)
		if err != nil {
			log.Printf("run error: %v", err)
		}
		if !worked {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func newTokenCodec(ctx context.Context) (*store.TokenCodec, error) {
	if key := os.Getenv("KMS_CRYPTO_KEY"); key != "" {
		envelope, err := platformcrypto.NewKMSEnvelope(ctx, key, os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
		if err != nil {
			return nil, err
		}
		return store.NewTokenCodec(envelope), nil
	}
	staticKey := mustEnv("CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64")
	envelope, err := platformcrypto.NewStaticEnvelope(staticKey)
	if err != nil {
		return nil, err
	}
	return store.NewTokenCodec(envelope), nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return v
}
