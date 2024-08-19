package internal

import "github.com/sandertv/gophertunnel/minecraft/protocol/login"

type Authenticator func(login.IdentityData, string) bool
