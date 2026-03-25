// obfuscate merges all pkg/core/*.go files into a single c0.go
// with private identifiers auto-detected and renamed to random short names.
// Also regenerates XOR-encoded licensing URL with a fresh key each build.
//
// Usage: go run obfuscate.go <core-dir> <output-file>
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const licensingURL = "https://license.evolutionfoundation.com.br"

// Names that must NOT be renamed (exported or used externally)
var keepNames = map[string]bool{
	"RuntimeContext":     true,
	"RuntimeConfig":     true,
	"InitializeRuntime": true,
	"GateMiddleware":    true,
	"ValidateContext":   true,
	"LicenseRoutes":     true,
	"StartHeartbeat":    true,
	"Shutdown":          true,
	"SetDB":             true,
	"MigrateDB":         true,
	"ComputeSessionSeed":  true,
	"ValidateRouteAccess": true,
	"DeriveInstanceToken": true,
	"ActivateIntegrity":   true,
	"ContextHash":       true,
	"IsActive":          true,
	"APIKey":            true,
	"InstanceID":        true,
	"RegistrationURL":   true,
	"TableName":         true,
	// Go builtins / common
	"init": true, "main": true, "error": true, "string": true,
	"byte": true, "int": true, "bool": true, "any": true,
	"uint": true, "uint64": true, "int64": true, "int32": true,
	"nil": true, "true": true, "false": true, "len": true,
	"make": true, "append": true, "copy": true, "close": true,
	"delete": true, "panic": true, "recover": true, "new": true,
	"range": true, "func": true, "type": true, "struct": true,
	"interface": true, "map": true, "chan": true, "select": true,
	"case": true, "default": true, "break": true, "continue": true,
	"return": true, "defer": true, "go": true, "for": true,
	"if": true, "else": true, "switch": true, "var": true,
	"const": true, "package": true, "import": true,
}

// Files to merge in order
var coreFiles = []string{
	"endpoint.go", "transport.go", "model.go",
	"store.go", "integrity.go", "runtime.go",
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <core-dir> <output-file>\n", os.Args[0])
		os.Exit(1)
	}

	coreDir := os.Args[1]
	outFile := os.Args[2]

	// ── Read all files ──
	var imports []string
	var bodies []string
	importSet := make(map[string]bool)
	importRe := regexp.MustCompile(`"([^"]+)"`)

	allContent := ""

	for _, f := range coreFiles {
		data, err := os.ReadFile(filepath.Join(coreDir, f))
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", f, err)
			continue
		}

		lines := strings.Split(string(data), "\n")
		inImport := false
		bodyStart := 0

		for i, line := range lines {
			trimmed := strings.TrimSpace(line)

			if trimmed == "package core" {
				continue
			}
			if strings.HasPrefix(trimmed, "import (") {
				inImport = true
				continue
			}
			if inImport {
				if trimmed == ")" {
					inImport = false
					bodyStart = i + 1
					continue
				}
				if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
					if !importSet[trimmed] {
						importSet[trimmed] = true
						imports = append(imports, "\t"+trimmed)
					}
				}
				continue
			}
			if strings.HasPrefix(trimmed, "import \"") {
				matches := importRe.FindStringSubmatch(trimmed)
				if len(matches) > 1 {
					imp := "\"" + matches[1] + "\""
					if !importSet[imp] {
						importSet[imp] = true
						imports = append(imports, "\t"+imp)
					}
				}
				bodyStart = i + 1
				continue
			}
			if bodyStart == 0 && trimmed != "" && !strings.HasPrefix(trimmed, "//") {
				bodyStart = i
			}
		}

		if bodyStart > 0 && bodyStart < len(lines) {
			body := strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
			if body != "" {
				bodies = append(bodies, body)
				allContent += body + "\n"
			}
		}
	}

	sort.Strings(imports)

	// ── Auto-detect private names ──
	privateNames := map[string]bool{}

	// Private functions: func name( or func (receiver) name(
	funcRe := regexp.MustCompile(`func\s+(?:\([^)]+\)\s+)?([a-z_][a-zA-Z0-9_]*)\s*\(`)
	for _, m := range funcRe.FindAllStringSubmatch(allContent, -1) {
		name := m[1]
		if len(name) > 2 && !keepNames[name] {
			privateNames[name] = true
		}
	}

	// Private vars/consts
	varRe := regexp.MustCompile(`(?:var|const)\s+([a-z_][a-zA-Z0-9_]*)\s`)
	for _, m := range varRe.FindAllStringSubmatch(allContent, -1) {
		name := m[1]
		if len(name) > 2 && !keepNames[name] {
			privateNames[name] = true
		}
	}

	// Struct fields (lowercase)
	fieldRe := regexp.MustCompile(`\t([a-z][a-zA-Z0-9_]*)\s+(?:string|int|bool|atomic\.|sync\.|time\.\w+|\[)`)
	for _, m := range fieldRe.FindAllStringSubmatch(allContent, -1) {
		name := m[1]
		if len(name) > 2 && !keepNames[name] {
			privateNames[name] = true
		}
	}

	// ── Generate random obfuscated names ──
	used := map[string]bool{}
	nameMap := map[string]string{}

	names := make([]string, 0, len(privateNames))
	for n := range privateNames {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		nameMap[name] = genName(used)
	}

	fmt.Fprintf(os.Stderr, "  Found %d private names to obfuscate\n", len(nameMap))

	// ── Generate fresh XOR key + encoded URL ──
	xorKey := make([]byte, len(licensingURL))
	rand.Read(xorKey)
	encodedURL := make([]byte, len(licensingURL))
	for i, b := range []byte(licensingURL) {
		encodedURL[i] = b ^ xorKey[i%len(xorKey)]
	}

	// ── Merge bodies ──
	merged := strings.Join(bodies, "\n\n")

	// Apply name obfuscation (longest first)
	keys := make([]string, 0, len(nameMap))
	for k := range nameMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	for _, k := range keys {
		// Only replace identifiers NOT inside string literals (quoted with ")
		// Match: word boundary + name + word boundary, but NOT preceded by " or followed by "
		re := regexp.MustCompile(`(?:^|[^"])(\b` + regexp.QuoteMeta(k) + `\b)(?:[^"]|$)`)
		merged = re.ReplaceAllStringFunc(merged, func(match string) string {
			// Check if the match is inside a quoted string by looking at context
			return strings.Replace(match, k, nameMap[k], 1)
		})
	}

	// Fix: restore any map keys that were incorrectly obfuscated inside string literals
	// Re-scan for patterns like `"_xyz":` and restore to original if it was a JSON/map key
	for orig, obf := range nameMap {
		// Restore obfuscated names inside string literals: "_xyz" → "original"
		merged = strings.ReplaceAll(merged, `"`+obf+`"`, `"`+orig+`"`)
	}

	// Replace XOR key/encoded URL literals in the merged code
	// Find the existing xorSeed and encodedURL declarations and replace with fresh ones
	xorSeedName := nameMap["xorSeed"]
	if xorSeedName == "" {
		xorSeedName = "_k1"
	}
	encodedURLName := nameMap["encodedURL"]
	if encodedURLName == "" {
		encodedURLName = "_k0"
	}

	// Remove comments
	commentRe := regexp.MustCompile(`(?m)^[ \t]*//.*\n`)
	merged = commentRe.ReplaceAllString(merged, "")

	// Collapse blank lines
	for strings.Contains(merged, "\n\n\n") {
		merged = strings.ReplaceAll(merged, "\n\n\n", "\n\n")
	}

	// ── Build output ──
	var out strings.Builder
	out.WriteString("package core\n\nimport (\n")
	for _, imp := range imports {
		out.WriteString(imp + "\n")
	}
	out.WriteString(")\n\n")

	// Write XOR vars at the top
	out.WriteString(fmt.Sprintf("var %s = []byte{%s}\n", xorSeedName, formatBytes(xorKey)))
	out.WriteString(fmt.Sprintf("var %s = []byte{%s}\n\n", encodedURLName, formatBytes(encodedURL)))

	// Remove original xorSeed and encodedURL declarations from merged
	// (they'll be replaced by our fresh ones above)
	xorDeclRe := regexp.MustCompile(`(?m)^var\s+` + regexp.QuoteMeta(xorSeedName) + `\s*=.*\n`)
	merged = xorDeclRe.ReplaceAllString(merged, "")
	encDeclRe := regexp.MustCompile(`(?ms)^var\s+` + regexp.QuoteMeta(encodedURLName) + `\s*=\s*func\(\).*?\n\}\(\)\s*\n`)
	merged = encDeclRe.ReplaceAllString(merged, "")

	out.WriteString(merged)
	out.WriteString("\n")

	if err := os.WriteFile(outFile, []byte(out.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  ✓ Generated %s (%d bytes, %d names obfuscated, XOR key refreshed)\n",
		outFile, out.Len(), len(nameMap))
}

func genName(used map[string]bool) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	for {
		length, _ := rand.Int(rand.Reader, big.NewInt(3))
		n := length.Int64() + 2 // 2-4 chars
		name := "_"
		for i := int64(0); i < n; i++ {
			idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
			name += string(chars[idx.Int64()])
		}
		if !used[name] {
			used[name] = true
			return name
		}
	}
}

func formatBytes(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("0x%s", hex.EncodeToString([]byte{v}))
	}
	return strings.Join(parts, ", ")
}
