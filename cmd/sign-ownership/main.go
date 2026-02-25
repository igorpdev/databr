// sign-ownership generates the EIP-191 ownership proof required by x402scan.
//
// It signs the origin URL ("https://databr.api.br") with the wallet private key
// using the same algorithm as MetaMask's personal_sign — EIP-191 prefixed keccak256.
//
// Usage (key read from stdin, not stored in shell history):
//
//	go run cmd/sign-ownership/main.go
//
// Then paste the raw hex private key (without 0x) and press Enter.
// The output is the signature to set as X402_OWNERSHIP_PROOF in Railway.
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

const origin = "https://databr.api.br"

func main() {
	fmt.Fprintf(os.Stderr, "Paste your wallet private key (hex, without 0x) and press Enter.\n")
	fmt.Fprintf(os.Stderr, "The key is read from stdin — it will NOT appear in shell history.\n\n")

	reader := bufio.NewReader(os.Stdin)
	keyHex, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading key: %v\n", err)
		os.Exit(1)
	}
	keyHex = strings.TrimSpace(keyHex)
	keyHex = strings.TrimPrefix(keyHex, "0x")

	privKey, err := crypto.HexToECDSA(keyHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid private key: %v\n", err)
		os.Exit(1)
	}

	// EIP-191 personal_sign hash: keccak256("\x19Ethereum Signed Message:\n" + len + message)
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(origin), origin)
	hash := crypto.Keccak256([]byte(msg))

	sig, err := crypto.Sign(hash, privKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "signing failed: %v\n", err)
		os.Exit(1)
	}

	// EIP-191: add 27 to recovery id (last byte) for Ethereum compatibility
	sig[64] += 27

	addr := crypto.PubkeyToAddress(privKey.PublicKey)
	signature := "0x" + hex.EncodeToString(sig)

	fmt.Fprintf(os.Stderr, "Wallet address : %s\n", addr.Hex())
	fmt.Fprintf(os.Stderr, "Signed message : %q\n\n", origin)
	fmt.Printf("X402_OWNERSHIP_PROOF=%s\n", signature)
}
