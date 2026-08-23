// Command dashboard-hash generates the bcrypt hash and session secret for
// the request-log dashboard.
//
//	go run ./cmd/dashboard-hash
//
// The password is read from the terminal without echoing, so it never
// reaches shell history or the process list. Copy the printed lines into
// .env — the plaintext password is never written anywhere.
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"gitlab.com/ariefhidayatulloh/easy-ig/reqlog"
)

func main() {
	pw, err := readPassword()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if len(pw) < 8 {
		fmt.Fprintln(os.Stderr, "error: use at least 8 characters")
		os.Exit(1)
	}

	hash, err := reqlog.HashPassword(pw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: hashing failed:", err)
		os.Exit(1)
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		fmt.Fprintln(os.Stderr, "error: generating session secret:", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Add these to .env (keep DASHBOARD_SESSION_SECRET stable, or")
	fmt.Println("everyone is logged out on each restart):")
	fmt.Println()
	fmt.Println("DASHBOARD_USER=admin")
	// Single quotes are REQUIRED, not cosmetic: godotenv expands $VAR in
	// unquoted values, and a bcrypt hash is full of dollar signs. Unquoted,
	// "$2a$10$..." silently loses characters and the hash no longer matches.
	fmt.Printf("DASHBOARD_PASSWORD_HASH='%s'\n", hash)
	fmt.Printf("DASHBOARD_SESSION_SECRET='%s'\n", hex.EncodeToString(secret))
	fmt.Println()
	fmt.Println("Keep the single quotes — .env expands $VAR in unquoted values,")
	fmt.Println("which would corrupt the dollar signs in the bcrypt hash.")
}

// readPassword prefers a no-echo terminal read; when stdin is piped it
// falls back to reading a line, which is what a scripted setup needs.
func readPassword() (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Dashboard password: ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		fmt.Fprint(os.Stderr, "Confirm password: ")
		b2, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		if string(b) != string(b2) {
			return "", fmt.Errorf("passwords do not match")
		}
		return string(b), nil
	}

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
