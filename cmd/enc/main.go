// enc provides passphrase-based encryption/decryption backed by AES-256 encryption
package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	var decrypt, pem, visual bool
	var filename, outfile, passphrase string
	flag.BoolVar(&pem, "b", false, "Use base64 encoding for input/output")
	flag.BoolVar(&decrypt, "d", false, "Decrypt file, default: encrypt")
	flag.StringVar(&filename, "f", "", "File path")
	flag.StringVar(&outfile, "o", "", "Override the file output path")
	flag.BoolVar(&visual, "v", false, "Print output to stdout")
	flag.Parse()

	if filename == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	fi, err := os.Stdin.Stat()
	if err != nil {
		log.Fatalf("failed to get os.Stdin: %v\n", err)
	}
	if fi.Mode()&os.ModeNamedPipe == 0 {
		passphrase = readPassword()
	} else {
		reader := bufio.NewReader(os.Stdin)
		var output []rune

		for {
			input, _, err := reader.ReadRune()
			if err != nil && err == io.EOF {
				break
			}
			output = append(output, input)
		}
		passphrase = strings.TrimSuffix(string(output), "\n") // Naive conversion to string, trim newlines

	}

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
	rf, e := ioutil.ReadFile(filename)
	if e != nil {
		log.Fatalf("failure to read file at %s :: %v\n", filename, e)
	}

	if decrypt {
		var f []byte
		if pem {
			var e error
			f, e = base64.URLEncoding.DecodeString(string(rf))
			if e != nil {
				log.Fatalf("unable to base64-decode file %s : %v \n", filename, e)
			}
		} else {
			f = rf
		}
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
			e := ioutil.WriteFile(outfile, plaintext, 0644)
			if e != nil {
				log.Fatalf("unable to write file: %v\n", e)
			}
		}
		if visual {
			// Write to stdout
			fmt.Fprintf(os.Stdout, "%s\n", plaintext)
		}

	} else {
		var f []byte
		f = rf // We're encoding so read the raw file

		nonce := make([]byte, gcm.NonceSize())
		if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
			log.Fatalf("unable to read random source: %v \n", err)
		}

		b := gcm.Seal(nonce, nonce, f, nil)
		format := "%s.enc"

		var fw []byte
		if pem { //Base64
			format = "%s.b64.enc"
			p := base64.URLEncoding.EncodeToString(b)
			if visual {
				fmt.Fprintf(os.Stdout, "%s\n", p)
				os.Exit(0)
			} else {
				fw = []byte(p)
			}
		} else {
			fw = b[:]
		}

		err = ioutil.WriteFile(fmt.Sprintf(format, filename), fw, 0644)
		if err != nil {
			log.Fatalf("unable to write encrypted file %s.enc :: %v \n", filename, err)
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
