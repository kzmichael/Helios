package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"math/big"
)

// rsaPublicKey adapts decoded JWKS components to crypto/rsa.PublicKey, which is
// what golang-jwt's RS256 verifier expects as the keyfunc's return value.
type rsaPublicKey struct {
	n *big.Int
	e int
}

func (k *rsaPublicKey) toStdlib() *rsa.PublicKey {
	return &rsa.PublicKey{N: k.n, E: k.e}
}

// decodeRSAComponents decodes the base64url "n" and "e" fields from a JWK.
func decodeRSAComponents(nb64, eb64 string) (*big.Int, int, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nb64)
	if err != nil {
		return nil, 0, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eb64)
	if err != nil {
		return nil, 0, err
	}
	if len(nBytes) == 0 || len(eBytes) == 0 {
		return nil, 0, errors.New("auth: empty RSA component")
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	return n, int(e.Int64()), nil
}
