package core

// Service endpoint configuration.
// In release builds, encodedEP and xorKey are set via ldflags:
//   -X github.com/EvolutionAPI/evolution-go/pkg/core.encodedEP=<hex>
//   -X github.com/EvolutionAPI/evolution-go/pkg/core.xorKey=<hex>
// This prevents the URL from appearing as a plain string in the binary.

var (
	// Set via ldflags in release builds. Fallback for dev mode below.
	encodedEP string
	xorKey    string
)

// resolveEndpoint decodes the service URL at runtime.
// Result is ephemeral — never stored in package-level variable.
func resolveEndpoint() string {
	if encodedEP != "" && xorKey != "" {
		return decodeXOR(encodedEP, xorKey)
	}
	// Dev fallback — assembled at runtime, not a single string literal
	parts := [...]string{"h", "tt", "ps", "://", "li", "ce", "nse", ".", "ev", "ol", "ut", "io", "nf", "ou", "nd", "at", "io", "n.", "co", "m.", "br"}
	var s string
	for _, p := range parts {
		s += p
	}
	return s
}

// decodeXOR decodes hex-encoded XOR data.
func decodeXOR(enc, key string) string {
	encBytes := hexDec(enc)
	keyBytes := hexDec(key)
	if len(keyBytes) == 0 {
		return ""
	}
	out := make([]byte, len(encBytes))
	for i, b := range encBytes {
		out[i] = b ^ keyBytes[i%len(keyBytes)]
	}
	return string(out)
}

// hexDec decodes a hex string without importing encoding/hex
// (avoids exposing the import in the symbol table).
func hexDec(s string) []byte {
	if len(s)%2 != 0 {
		return nil
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		b[i/2] = hexVal(s[i])<<4 | hexVal(s[i+1])
	}
	return b
}

func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}
