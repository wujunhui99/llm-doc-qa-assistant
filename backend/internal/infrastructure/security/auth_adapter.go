package security

import cryptauth "llm-doc-qa-assistant/backend/internal/auth"

type PasswordHasher struct{}

func (PasswordHasher) HashPassword(password string) (string, error) {
	return cryptauth.HashPassword(password)
}

func (PasswordHasher) VerifyPassword(password, encoded string) bool {
	return cryptauth.VerifyPassword(password, encoded)
}

type TokenGenerator struct{}

func (TokenGenerator) NewSessionToken() (string, error) {
	return cryptauth.NewSessionToken()
}
