package token

import "github.com/huwenlong92/sdkit-go/pkg/security/crypto"

func RandomToken(bytes int) (string, error) {
	return crypto.RandomBase64URL(bytes)
}
