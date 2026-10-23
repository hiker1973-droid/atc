// licensekey makes SkyEye ATC licence keys. It is for the operator's PC only:
// it is not in SkyEye-Setup.exe, and the private key it uses never goes in the
// repo (operator 2026-10-23: "build licsencing and 14 day demo").
//
//	licensekey init                       make the signing key pair (once)
//	licensekey issue -squadron "VFA-103" [-days 365] [-id ...]
//	licensekey check <key>                say what a key holds
//
// The private key file defaults to %USERPROFILE%\SkyEye-licensing\signing-key.txt.
// init prints the public key for pkg/license/pubkey.go; a key signed by a
// private key whose public half is not built in is rejected.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vsfg7/atc/pkg/license"
)

func defaultKeyFile() string {
	return filepath.Join(os.Getenv("USERPROFILE"), "SkyEye-licensing", "signing-key.txt")
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = initKeys(os.Args[2:])
	case "issue":
		err = issue(os.Args[2:])
	case "check":
		err = check(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "licensekey:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `licensekey init | issue -squadron NAME [-days N] [-id ID] | check KEY`)
	os.Exit(2)
}

func initKeys(args []string) error {
	fl := flag.NewFlagSet("init", flag.ExitOnError)
	file := fl.String("key", defaultKeyFile(), "private key file to create")
	fl.Parse(args)
	if _, err := os.Stat(*file); err == nil {
		return fmt.Errorf("%s already exists - keys signed with it would stop working if it were replaced", *file)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*file), 0o700); err != nil {
		return err
	}
	text := "SkyEye ATC licence signing key - PRIVATE. Keep a backup somewhere safe; never share or commit it.\r\n" +
		base64.StdEncoding.EncodeToString(priv) + "\r\n"
	if err := os.WriteFile(*file, []byte(text), 0o600); err != nil {
		return err
	}
	fmt.Printf("Private key written to %s\n\nPublic key for pkg/license/pubkey.go:\n%s\n", *file, hex.EncodeToString(pub))
	return nil
}

func loadPriv(file string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("no signing key (%v) - run licensekey init first", err)
	}
	lines := strings.Fields(string(b))
	raw, err := base64.StdEncoding.DecodeString(lines[len(lines)-1])
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%s is not a signing key", file)
	}
	return ed25519.PrivateKey(raw), nil
}

func issue(args []string) error {
	fl := flag.NewFlagSet("issue", flag.ExitOnError)
	file := fl.String("key", defaultKeyFile(), "private key file")
	squadron := fl.String("squadron", "", "who the licence is for (shown on their Settings page)")
	days := fl.Int("days", 0, "days until it ends; 0 = never")
	id := fl.String("id", "", "licence id (default: date and random)")
	fl.Parse(args)
	if strings.TrimSpace(*squadron) == "" {
		return fmt.Errorf("give -squadron")
	}
	priv, err := loadPriv(*file)
	if err != nil {
		return err
	}
	if !priv.Public().(ed25519.PublicKey).Equal(license.PublicKey) {
		return fmt.Errorf("this signing key is not the one built into SkyEye (pkg/license/pubkey.go): its keys would be rejected")
	}
	now := time.Now().UTC()
	l := license.Licence{Squadron: strings.TrimSpace(*squadron), ID: *id, Issued: now.Format("2006-01-02")}
	if l.ID == "" {
		r := make([]byte, 3)
		rand.Read(r)
		l.ID = now.Format("20060102") + "-" + hex.EncodeToString(r)
	}
	if *days > 0 {
		l.Expires = now.AddDate(0, 0, *days).Format("2006-01-02")
	}
	key, err := license.Sign(l, priv)
	if err != nil {
		return err
	}
	exp := "never"
	if l.Expires != "" {
		exp = l.Expires
	}
	fmt.Fprintf(os.Stderr, "Licence %s for %s, ends %s:\n", l.ID, l.Squadron, exp)
	fmt.Println(key)
	return nil
}

func check(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("give the key")
	}
	l, err := license.Parse(strings.Join(args, ""), license.PublicKey)
	if err != nil {
		return err
	}
	exp := "never"
	if l.Expires != "" {
		exp = l.Expires
	}
	fmt.Printf("Valid: %s, id %s, issued %s, ends %s\n", l.Squadron, l.ID, l.Issued, exp)
	return nil
}
