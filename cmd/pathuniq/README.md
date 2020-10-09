# pathuniq

How to use:

```go get -u github.com/tydavis/utilities/cmd/pathuniq```

then add

```eval <path to pathuniq>```

to the bottom of your shell RC file (usually `~/.bashrc` or `~/.zshrc`)

## How it works

1. Get the PATH variable
1. Split by os-specific path separator (OSX/*nix or Windows)
1. Rebuild array with only unique values, preserving order
1. Print the output as a shell command for the parent shell (bash, cmd, etc) to evaluate
