package util

import "github.com/sandertv/gophertunnel/minecraft/protocol/login"

type Authentication interface {
	Authenticate(login.IdentityData, string) bool
}

type SecretBasedAuthentication struct {
	secret string
}

func NewSecretBasedAuthentication(secret string) *SecretBasedAuthentication {
	return &SecretBasedAuthentication{secret: secret}
}

func (authentication *SecretBasedAuthentication) Authenticate(_ login.IdentityData, token string) bool {
	return authentication.secret == token
}
