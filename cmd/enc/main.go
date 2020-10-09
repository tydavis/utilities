// enc provides passphrase-based encryption/decryption backed by AES-256 encryption
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	var decrypt bool
	var filename, outfile, passphrase string
	flag.BoolVar(&decrypt, "d", false, "Decrypt file, default: encrypt")
	flag.StringVar(&filename, "f", "", "File path")
	flag.StringVar(&outfile, "o", "", "Override the file output path")
	flag.Parse()

	if filename == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	passphrase = readPassword()

	hasher := sha256.New()
	hasher.Write([]byte(passphrase))
	khash := hasher.Sum(nil)
	key := khash[:]

	// "NewCipher creates and returns a new cipher.Block. The key argument
	// should be the AES key, either 16, 24, or 32 bytes to select AES-128,
	// AES-192, or AES-256."
	// Key length in this case is 32, so AES-256
	c, err := aes.NewCipher(key)
	if err != nil {
		log.Fatalln(err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		log.Fatalln(err)
	}
	f, e := ioutil.ReadFile(filename)
	if e != nil {
		log.Fatalf("failure to read file at %s :: %v\n", filename, e)
	}

	if decrypt {
		nonceSize := gcm.NonceSize()
		if len(f) < nonceSize {
			log.Fatalf("file smaller than nonceSize, breaking AES: %v\n", err)
		}

		nonce, ciphertext := f[:nonceSize], f[nonceSize:]
		plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			log.Fatalf("failure to decrypt: %v\n", err)
		}

		if outfile != "" {
			e := ioutil.WriteFile(outfile, plaintext, 0755)
			if e != nil {
				log.Fatalf("unable to write file: %v\n", e)
			}
		} else {
			// Write to stdout
			fmt.Fprintf(os.Stdout, "%s\n", plaintext)
		}

	} else {
		nonce := make([]byte, gcm.NonceSize())
		if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
			log.Fatalln(err)
		}
		err = ioutil.WriteFile(fmt.Sprintf("%s.enc", filename), gcm.Seal(nonce, nonce, f, nil), 0777)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

// readPassword reads password without echoing to the terminal
func readPassword() string {
	fmt.Print("enter passphrase: ")
	var bytePassword []byte
	var err error
	bytePassword, err = terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatalf("Cannot read passphrase from terminal :: %v", err)
	}
	fmt.Println()
	return string(bytePassword)
}
