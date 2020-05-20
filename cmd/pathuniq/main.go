package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

func uniqAdd(s []string, e string) []string {
	for _, n := range s {
		if n == e {
			return s
		}
	}
	return append(s, e)
}

func main() {
	p := os.Getenv("PATH")
	ps := strings.Split(p, string(os.PathListSeparator))
	var pu []string
	for _, k := range ps {
		pu = uniqAdd(pu, k)
	}
	np := strings.Join(pu, string(os.PathListSeparator))

	// Cannot export value to shell, must evaluate in shell
	switch runtime.GOOS {
	case "windows":
		fmt.Println("set PATH=" + np)
	default:
		fmt.Println("export PATH=" + np)
	}
}
