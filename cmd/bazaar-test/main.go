package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	x402sdk "github.com/coinbase/x402/go"
	x402http "github.com/coinbase/x402/go/http"
	evmexact "github.com/coinbase/x402/go/mechanisms/evm/exact/client"
	"github.com/coinbase/x402/go/signers/evm"
)

func main() {
	privateKey := os.Getenv("WALLET_PRIVATE_KEY")
	if privateKey == "" {
		log.Fatal("WALLET_PRIVATE_KEY env var required")
	}

	baseURL := os.Getenv("DATABR_URL")
	if baseURL == "" {
		baseURL = "https://databr-production.up.railway.app"
	}

	// Create EVM signer from private key
	signer, err := evm.NewClientSignerFromPrivateKey(privateKey)
	if err != nil {
		log.Fatalf("signer: %v", err)
	}
	fmt.Printf("Wallet address: %s\n", signer.Address())

	// Create x402 client with EVM exact scheme for Base mainnet
	x402Client := x402sdk.Newx402Client()
	evmScheme := evmexact.NewExactEvmScheme(signer)
	x402Client.Register("eip155:8453", evmScheme)

	// Wrap standard HTTP client with x402 payment handling
	httpClient := x402http.WrapHTTPClientWithPayment(http.DefaultClient, x402http.NewClient(x402Client))

	url := baseURL + "/v1/bcb/cambio/USD"
	fmt.Printf("Requesting: %s\n", url)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("X-PAYMENT-RESPONSE: %s\n", resp.Header.Get("X-PAYMENT-RESPONSE"))
	fmt.Printf("Body: %s\n", string(body))
}
