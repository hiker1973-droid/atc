package license

import (
	"crypto/ed25519"
	"encoding/hex"
)

// PublicKey checks licence keys. Its private half was made by `licensekey
// init` on the operator's PC and is kept there; replacing this invalidates
// every key issued so far.
var PublicKey = ed25519.PublicKey(mustHex("865d5b634f0ced9de58f829c22000ca1967972d1aaedd01d5b302f59a6775d1e"))

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != ed25519.PublicKeySize {
		return make([]byte, ed25519.PublicKeySize) // no key built in: every key is rejected
	}
	return b
}
