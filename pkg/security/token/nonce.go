package token

import "github.com/huwenlong92/sdkit-go/pkg/security/crypto"

func Nonce() (string, error) {
	return crypto.RandomHex(16)
}
