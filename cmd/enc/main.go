// enc provides passphrase-based encryption/decryption backed by AES-256 encryption
package main

import (
	"bufio"
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
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	var decrypt, visual bool
	var filename, outfile, passphrase string
	flag.BoolVar(&decrypt, "d", false, "Decrypt file, default: encrypt")
	flag.StringVar(&filename, "f", "", "File path")
	flag.StringVar(&outfile, "o", "", "Override the file output path")
	flag.BoolVar(&visual, "v", false, "Print output to stdout")
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

		if visual {
			// Write to stdout
			fmt.Fprintf(os.Stdout, "%s\n", plaintext)
		} else {
			var of string
			if outfile == "" {
				// Default name is `filename` without the ".enc" suffix
				of = strings.TrimSuffix(filename, ".enc")
				if strings.EqualFold(of, filename) {
					of = filename + ".dec" // "Decoded" file format
				}
			}
			e := ioutil.WriteFile(of, plaintext, 0644)
			if e != nil {
				log.Fatalf("unable to write file: %v\n", e)
			}
		}

	} else {
		nonce := make([]byte, gcm.NonceSize())
		if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
			log.Fatalf("unable to read random source: %v \n", err)
		}

		b := gcm.Seal(nonce, nonce, f, nil)
		format := "%s.enc"
		err = ioutil.WriteFile(fmt.Sprintf(format, filename), b, 0644)
		if err != nil {
			log.Fatalf("unable to write encrypted file %s.enc :: %v \n", filename, err)
		}
	}
}

// readPassword reads password without echoing to the terminal
func readPassword() string {
	var fd int
	var pass string
	if terminal.IsTerminal(syscall.Stdin) {
		fmt.Fprint(os.Stderr, "enter passphrase: ")
		fd = syscall.Stdin
		inputPass, err := terminal.ReadPassword(fd)
		if err != nil {
			log.Fatalf("Cannot read passphrase from terminal :: %v", err)
		}
		pass = string(inputPass)
		fmt.Println()
	} else {
		var err error
		reader := bufio.NewReader(os.Stdin)
		pass, err = reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Cannot read passphrase from stdin/pipe: %v", err)
		}
	}
	return pass
}
