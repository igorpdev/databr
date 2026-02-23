package x402

import (
	"context"
	"net/http"
)

// walletCtxKey is the context key for the payer wallet address injected after x402 settlement.
type walletCtxKey struct{}

// WalletFromRequest returns the payer wallet address set by the x402 middleware
// after successful settlement. Returns "" if no wallet is present (e.g. dev mode,
// no payment, or pre-settlement).
func WalletFromRequest(r *http.Request) string {
	if v, ok := r.Context().Value(walletCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// injectWallet returns a copy of r with the given wallet address in its context.
// Used by the x402 middleware after successful settlement to pass the payer identity
// to downstream handlers and rate limiters.
func injectWallet(r *http.Request, wallet string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), walletCtxKey{}, wallet))
}
