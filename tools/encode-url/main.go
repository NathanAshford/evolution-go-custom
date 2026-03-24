// encode-url generates XOR-encoded URL + key for ldflags.
// Usage: go run ./tools/encode-url "https://license.evolutionfoundation.com.br"
// Output: two hex strings (encoded URL and XOR key) for use with -ldflags.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <url>\n", os.Args[0])
		os.Exit(1)
	}

	url := []byte(os.Args[1])
	key := make([]byte, len(url))
	if _, err := rand.Read(key); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate key: %v\n", err)
		os.Exit(1)
	}

	encoded := make([]byte, len(url))
	for i, b := range url {
		encoded[i] = b ^ key[i]
	}

	fmt.Printf("ENCODED=%s\n", hex.EncodeToString(encoded))
	fmt.Printf("XOR_KEY=%s\n", hex.EncodeToString(key))
}
