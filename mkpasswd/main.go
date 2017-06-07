package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	fmt.Print("Token: ")
	token, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		panic(err)
	}
	t := sha256.Sum256([]byte(token))
	b64 := base64.StdEncoding.EncodeToString(t[:])
	fmt.Print(`\nadded to "token" in server.json: `, b64, "\n")
}
