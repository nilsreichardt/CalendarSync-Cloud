package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type oauthState struct {
	UserID string `json:"userId"`
	Issued int64  `json:"issued"`
	Nonce  string `json:"nonce"`
}

func (s *Server) signState(userID string) (string, error) {
	payload := oauthState{
		UserID: userID,
		Issued: time.Now().Unix(),
		Nonce:  nowRFC3339(),
	}
	by, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(by)
	mac := hmac.New(sha256.New, s.stateSecret)
	mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return body + "." + sig, nil
}

func (s *Server) verifyState(raw string) (*oauthState, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid state format")
	}
	mac := hmac.New(sha256.New, s.stateSecret)
	mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return nil, errors.New("invalid state signature")
	}
	by, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var payload oauthState
	if err := json.Unmarshal(by, &payload); err != nil {
		return nil, err
	}
	if time.Since(time.Unix(payload.Issued, 0)) > 15*time.Minute {
		return nil, errors.New("state expired")
	}
	return &payload, nil
}
