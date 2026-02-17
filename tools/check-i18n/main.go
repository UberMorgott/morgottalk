// check-i18n checks that all languages in i18n.ts have the same keys as English.
//
// Usage: go run ./tools/check-i18n [--path frontend/src/lib/i18n.ts]
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

var (
	// Matches a language block opening: "  en: {" or "  zh: {"
	langBlockRe = regexp.MustCompile(`^\s{2}(\w+)\s*:\s*\{`)
	// Matches a key line: "    someKey: "value"," or "    some_key: 'value',"
	// Also handles keys without trailing comma (last key in block).
	keyLineRe = regexp.MustCompile(`^\s{4}(\w+)\s*:`)
	// Matches a block closing: "  }," or "  }"
	blockCloseRe = regexp.MustCompile(`^\s{2}\}`)
	// Matches a comment-only line inside a block
	commentRe = regexp.MustCompile(`^\s*//`)
)

func main() {
	path := flag.String("path", "frontend/src/lib/i18n.ts", "path to i18n.ts file")
	flag.Parse()

	langs, err := parseI18n(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	enKeys, ok := langs["en"]
	if !ok {
		fmt.Fprintln(os.Stderr, "error: 'en' language block not found")
		os.Exit(1)
	}

	// Build sorted list of language codes (excluding en).
	var langCodes []string
	for code := range langs {
		if code != "en" {
			langCodes = append(langCodes, code)
		}
	}
	sort.Strings(langCodes)

	enSet := toSet(enKeys)
	hasErrors := false

	for _, code := range langCodes {
		keys := langs[code]
		otherSet := toSet(keys)

		var missing, extra []string
		for k := range enSet {
			if !otherSet[k] {
				missing = append(missing, k)
			}
		}
		for k := range otherSet {
			if !enSet[k] {
				extra = append(extra, k)
			}
		}
		sort.Strings(missing)
		sort.Strings(extra)

		if len(missing) > 0 {
			hasErrors = true
			fmt.Printf("%s: %d missing key(s):\n", code, len(missing))
			for _, k := range missing {
				fmt.Printf("  - %s\n", k)
			}
		}
		if len(extra) > 0 {
			hasErrors = true
			fmt.Printf("%s: %d extra key(s) not in en:\n", code, len(extra))
			for _, k := range extra {
				fmt.Printf("  + %s\n", k)
			}
		}
	}

	if hasErrors {
		fmt.Printf("\nen has %d keys, checked %d language(s)\n", len(enKeys), len(langCodes))
		os.Exit(1)
	}

	fmt.Printf("OK: all %d language(s) match en (%d keys)\n", len(langCodes), len(enKeys))
}

// parseI18n reads the i18n.ts file and returns a map of language code -> ordered list of keys.
func parseI18n(path string) (map[string][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	langs := make(map[string][]string)
	var currentLang string
	inBlock := false
	depth := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip comment-only lines.
		if commentRe.MatchString(line) {
			continue
		}

		if !inBlock {
			if m := langBlockRe.FindStringSubmatch(line); m != nil {
				currentLang = m[1]
				inBlock = true
				depth = 1
				continue
			}
		} else {
			// Track nested braces (for safety, though i18n.ts is flat).
			depth += strings.Count(line, "{") - strings.Count(line, "}")
			if depth <= 0 || blockCloseRe.MatchString(line) {
				inBlock = false
				currentLang = ""
				depth = 0
				continue
			}

			if m := keyLineRe.FindStringSubmatch(line); m != nil {
				langs[currentLang] = append(langs[currentLang], m[1])
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(langs) == 0 {
		return nil, fmt.Errorf("no language blocks found in %s", path)
	}

	return langs, nil
}

func toSet(keys []string) map[string]bool {
	s := make(map[string]bool, len(keys))
	for _, k := range keys {
		s[k] = true
	}
	return s
}
