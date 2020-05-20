package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

var verbose bool

type repo struct {
	path    string
	remotes map[string]string
}

func main() {
	confpath := flag.String("c", "~/.setup/gitlist", "Config file containing all git repos and remotes")
	workDir := flag.String("d", "~/code", "Root directory to perform clones and updates")
	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.Parse()

	h, e := buildchdir(*workDir)
	if e != nil {
		fmt.Printf("unable to create or chdir to: %s\n", *workDir)
		fmt.Printf("error: %v", e)
		os.Exit(1)
	}

	gitpath, err := exec.LookPath("git")
	if err != nil {
		fmt.Printf("unable to find git binary: %v\n", err)
		os.Exit(1)
	}

	// load config
	gl, err := loadConf(*confpath)
	if err != nil {
		fmt.Printf("conf file error: %v \n", err)
		os.Exit(1)
	}
	if verbose {
		fmt.Printf("config data: \n %+v \n", gl)
	}

	// Walk the list of repos in gitlist
	for i := range gl {
		wd := filepath.Join(h, gl[i].path)
		if stat, err := os.Stat(wd); err != nil || !stat.IsDir() { // Repo not found
			err := cloneRepo(gitpath, h, gl[i])
			if err != nil {
				fmt.Printf("failed to clone repository at path: %s :: %v\n", gl[i].path, err)
				continue
			}
			err = updateRemotes(gitpath, h, gl[i])
			if err != nil {
				fmt.Printf("failed to add remotes to new clone: %v\n", err)
				continue
			}
		} else {
			err := updateRemotes(gitpath, h, gl[i])
			if err != nil {
				fmt.Printf("failed to update remotes: %v\n", err)
				continue
			}
		}
	}
}

func cloneRepo(gp, h string, r repo) error {
	cmd := exec.Command(gp, "clone", r.remotes["origin"], filepath.Join(h, r.path))
	_, cerr := cmd.CombinedOutput()
	return cerr
}

func updateRemotes(gp, h string, r repo) error {
	wd := filepath.Join(h, r.path)
	err := os.Chdir(wd)
	if err != nil {
		fmt.Printf("failed to chdir: %s : %v\n", wd, err)
		return err
	}
	defer os.Chdir(h) //nolint:errcheck

	// Gather current list of remotes to compare to map
	cmd := exec.Command(gp, "remote")
	res, cerr := cmd.CombinedOutput()
	if cerr != nil {
		return cerr
	}

	local := strings.Split(string(res), "\n")
	// Add any remotes that don't already exist
	for k, v := range r.remotes {
		if contains(local, k) {
			continue
		}
		gadd := exec.Command(gp, "remote", "add", k, v)
		_, err := gadd.CombinedOutput()
		if err != nil {
			fmt.Printf("failed to add remote %s=%s", k, v)
			return err
		}
	}

	// Check existing remotes match our info
	for _, v := range local {
		if v == "" {
			continue
		}

		gc := exec.Command(gp, "config", "--get", fmt.Sprintf("remote.%s.url", v))
		resp, e := gc.CombinedOutput()
		if e != nil {
			fmt.Printf("failed to get remote url: %s, %v\n", v, e)
			continue
		}
		m, ok := r.remotes[v]
		if !ok {
			if verbose {
				fmt.Printf("new remote found: %s=%s @ %s\n", v, resp, wd)
			}
			continue
		}
		if strings.TrimSpace(string(resp)) != m {
			if verbose {
				fmt.Printf("remote does not match: %s %s", wd, resp)
			}
			gset := exec.Command(gp, "remote", "set-url", v, m)
			_, err := gset.CombinedOutput()
			if err != nil {
				fmt.Printf("failed to set url: %s\n", m)
				return err
			}

		}
	}

	return nil
}

type repolist []repo

func (a repolist) Len() int           { return len(a) }
func (a repolist) Less(i, j int) bool { return a[i].path < a[j].path }
func (a repolist) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// loadConf loads and parses the configuration file as passed in,
// returning the gitlist struct and any errors
func loadConf(cpath string) ([]repo, error) {
	fp, e := parsePath(cpath)
	if e != nil {
		return []repo{}, e
	}

	_, err := os.Stat(fp)
	if os.IsNotExist(err) {
		return []repo{}, err
	}

	f, e := os.Open(fp)
	if e != nil {
		return []repo{}, e
	}
	defer f.Close()

	list := make([]repo, 0, 100) // prealloc capacity for 100 repos
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		cline := strings.Split(scanner.Text(), ",")
		r := repo{
			path: cline[0],
		}
		r.remotes = make(map[string]string)
		for _, l := range cline[1:] {
			sp := strings.Split(l, "=")
			r.remotes[sp[0]] = sp[1]
		}
		list = append(list, r)
	}
	sort.Sort(repolist(list)) // Return sorted
	return list, scanner.Err()
}

// parsePath ensures normalization of file paths,
// expanded to the home directory if tilde (~) is present
func parsePath(cpath string) (fullpath string, err error) {
	if strings.HasPrefix(cpath, "~") {
		var d string
		d, err = os.UserHomeDir()
		if err != nil {
			return
		}
		fullpath = path.Clean(strings.ReplaceAll(cpath, "~", d))
		return
	}
	fullpath = path.Clean(cpath)
	return
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func buildchdir(workdir string) (fullpath string, err error) {
	var h string
	if workdir == "" { // Someone intentionally passes an empty string
		h, err = os.UserHomeDir()
		if err != nil {
			fmt.Printf("could not find home directory for user: %s\n", workdir)
			os.Exit(1)
		}
		fullpath = filepath.Join(h, "code") // Our default
	} else {
		h, err = parsePath(workdir)
		if err != nil {
			fmt.Printf("could not parse declared directory: %s\n", workdir)
			return
		}
	}
	fullpath = h

	// Check to ensure the directory exists
	d, e := os.Stat(fullpath)
	if os.IsNotExist(e) {
		merr := os.MkdirAll(fullpath, 0777)
		if merr != nil {
			fmt.Printf("failed to mkdir %s\n", h)
			err = merr
			return
		}
	} else if !d.IsDir() {
		err = errors.New("path is not a directory")
		return
	}

	herr := os.Chdir(fullpath) // Forcibly change directory to active workdir
	if herr != nil {
		fmt.Printf("failed to chdir: %v\n", herr)
		return
	}
	return
}
