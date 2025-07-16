package installer

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
)

const codeSigningKeyURL = "https://mullvad.net/media/mullvad-code-signing.asc"

func verifyPGP(file, sigURL string) error {
	cli := &http.Client{Timeout: 10 * time.Second}

	pubResp, err := cli.Get(codeSigningKeyURL)
	if err != nil {
		return fmt.Errorf("get key: %w", err)
	}
	defer pubResp.Body.Close()
	if pubResp.StatusCode != 200 {
		return fmt.Errorf("key status %d", pubResp.StatusCode)
	}
	keyring, err := openpgp.ReadArmoredKeyRing(pubResp.Body)
	if err != nil {
		return fmt.Errorf("parse key: %w", err)
	}

	sigResp, err := cli.Get(sigURL)
	if err != nil {
		return fmt.Errorf("get sig: %w", err)
	}
	defer sigResp.Body.Close()
	if sigResp.StatusCode != 200 {
		return fmt.Errorf("sig status %d", sigResp.StatusCode)
	}

	blk, err := armor.Decode(sigResp.Body)
	if err != nil {
		return fmt.Errorf("decode sig: %w", err)
	}
	if blk.Type != "PGP SIGNATURE" {
		return fmt.Errorf("unexpected block %q", blk.Type)
	}

	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	if _, err := openpgp.CheckDetachedSignature(keyring, f, blk.Body); err != nil {
		return fmt.Errorf("signature invalid: %w", err)
	}
	return nil
}
