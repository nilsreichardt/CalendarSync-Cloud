package main

import (
	"context"
	"log"
	"net/http"
	"os"

	platformapi "github.com/inovex/CalendarSync/internal/platform/api"
	platformcrypto "github.com/inovex/CalendarSync/internal/platform/crypto"
	"github.com/inovex/CalendarSync/internal/platform/store"
)

func main() {
	ctx := context.Background()
	tokenCodec, err := newTokenCodec(ctx)
	if err != nil {
		log.Fatal(err)
	}

	srv, err := platformapi.NewServer(ctx, platformapi.Config{
		DatabaseURL:         mustEnv("DATABASE_URL"),
		GoogleClientID:      mustEnv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleClientSecret:  mustEnv("GOOGLE_OAUTH_CLIENT_SECRET"),
		GoogleRedirectURL:   mustEnv("GOOGLE_OAUTH_REDIRECT_URL"),
		OAuthStateSecretB64: mustEnv("OAUTH_STATE_SECRET_B64"),
		SchedulerSecret:     mustEnv("SCHEDULER_SHARED_SECRET"),
		FrontendSecret:      os.Getenv("FRONTEND_SHARED_SECRET"),
	}, tokenCodec)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := srv.Close(); err != nil {
			log.Printf("server close error: %v", err)
		}
	}()

	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}
	log.Printf("calendarsync-api listening on :%s", addr)
	log.Fatal(http.ListenAndServe(":"+addr, srv.Handler()))
}

func newTokenCodec(ctx context.Context) (*store.TokenCodec, error) {
	if key := os.Getenv("KMS_CRYPTO_KEY"); key != "" {
		envelope, err := platformcrypto.NewKMSEnvelope(ctx, key, os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
		if err != nil {
			return nil, err
		}
		staticKey := os.Getenv("CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64")
		if staticKey == "" {
			return store.NewTokenCodec(envelope), nil
		}
		legacy, err := platformcrypto.NewStaticEnvelope(staticKey)
		if err != nil {
			return nil, err
		}
		return store.NewTokenCodecWithLegacy(envelope, legacy), nil
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
