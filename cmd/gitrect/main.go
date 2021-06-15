package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/karrick/godirwalk"
)

var (
	verbose bool
	debug   bool
)

// Repo provides the file Path and corresponding git Remotes for each repository
type Repo struct {
	Path    string            `json:"path"`
	Remotes map[string]string `json:"remotes"`
}

// Repolist wraps the slice of repos for JSON serialization
type Repolist struct {
	Repos []Repo `json:"repos"`
}

func (a Repolist) Len() int           { return len(a.Repos) }
func (a Repolist) Less(i, j int) bool { return a.Repos[i].Path < a.Repos[j].Path }
func (a Repolist) Swap(i, j int)      { a.Repos[i], a.Repos[j] = a.Repos[j], a.Repos[i] }

func main() {
	update := flag.Bool("u", false, "Update gitlist")
	confpath := flag.String("c", "~/.setup/gitlist", "Config file containing all git repos and remotes")
	workDir := flag.String("d", "~/code", "Root directory to perform clones and updates")
	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.BoolVar(&debug, "debug", false, "debug-level output")
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

	cpath, err := parsePath(*confpath)
	if err != nil {
		fmt.Printf("unable to parse config path: %v", err)
		os.Exit(1)
	}

	if *update {
		// Do updates
		rlist, err := visit(h)
		if err != nil {
			fmt.Printf("failed to update code: %v\n", err)
			os.Exit(1)
		}
		r := Repolist{Repos: rlist}
		r.getRemotes(gitpath, h)

		if _, err := os.Stat(cpath); err == nil { // The file exists, but we want to overwrite it
			err := os.Remove(cpath)
			if err != nil {
				fmt.Printf("failed to delete config file: %v", err)
				os.Exit(1)
			}
		}
		jf, err := os.Create(cpath)
		if err != nil {
			fmt.Printf("unable to open file %s :: %v", *confpath, err)
			os.Exit(1)
		}
		defer jf.Close() //nolint:errcheck

		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			fmt.Printf("json encoding error: %v", err)
			os.Exit(1)
		}
		if _, err := jf.Write(b); err != nil {
			fmt.Printf("file write error: %v", err)
			os.Exit(1)
		}

		// Maybe switch to streaming once files get large?
		//enc := json.NewEncoder(jf)
		//if err := enc.Encode(&r); err != nil {
		//	fmt.Printf("json encoding error: %v", err)
		//	os.Exit(1)
		//}
	} else {
		// load config
		gl, err := loadConf(*confpath)
		if err != nil {
			fmt.Printf("conf file error: %v \n", err)
			os.Exit(1)
		}
		if debug {
			fmt.Printf("config data: \n %+v \n", gl)
		}

		// Walk the list of repos in gitlist
		for i := range gl.Repos {
			wd := filepath.Join(h, gl.Repos[i].Path)
			if stat, err := os.Stat(wd); err != nil || !stat.IsDir() { // Repo not found
				if verbose {
					fmt.Printf("cloning repo: %s\n", gl.Repos[i].Path)
				}
				err := cloneRepo(gitpath, h, gl.Repos[i])
				if err != nil {
					fmt.Printf("failed to clone repository at path: %s :: %v\n", gl.Repos[i].Path, err)
					continue
				}
				err = updateRemotes(gitpath, h, gl.Repos[i])
				if err != nil {
					fmt.Printf("failed to add remotes to new clone: %v\n", err)
					continue
				}
			} else {
				if verbose {
					fmt.Printf("updating remotes for: %s\n", gl.Repos[i].Path)
				}
				err := updateRemotes(gitpath, h, gl.Repos[i])
				if err != nil {
					fmt.Printf("failed to update remotes: %v\n", err)
					continue
				}
			}
		}
	}
}

func cloneRepo(gp, h string, r Repo) error {
	cmd := exec.Command(gp, "clone", r.Remotes["origin"], filepath.Join(h, r.Path))
	_, cerr := cmd.CombinedOutput()
	return cerr
}

func updateRemotes(gp, h string, r Repo) error {
	wd := filepath.Join(h, r.Path)
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
	for k, v := range r.Remotes {
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
		m, ok := r.Remotes[v]
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

// getRemotes iterates through a repolist to add remotes to all identified repos
func (a *Repolist) getRemotes(gp, h string) {
	for i, r := range a.Repos {
		a.Repos[i].Remotes = make(map[string]string, 3) // Prebuild map to avoid nil assignment

		wd := filepath.Join(h, r.Path)
		err := os.Chdir(wd)
		if err != nil {
			fmt.Printf("error: failed to chdir: %s : %v\n", wd, err)
			os.Chdir(h) //nolint:errcheck
			continue
		}

		// Gather current list of remotes to compare to map
		cmd := exec.Command(gp, "remote")
		re, cerr := cmd.CombinedOutput()
		if cerr != nil {
			fmt.Printf("failed to gather remotes for: %s :: %v\n", r.Path, cerr)
			break
		}

		local := strings.Split(string(re), "\n")
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
			rem := strings.Split(string(resp), "\n")[0]
			a.Repos[i].Remotes[v] = string(rem)

		}
		os.Chdir(h) //nolint:errcheck
	}
}

// loadConf loads and parses the configuration file as passed in,
// returning the gitlist struct and any errors
func loadConf(cpath string) (Repolist, error) {
	var r Repolist
	fp, e := parsePath(cpath)
	if e != nil {
		return r, e
	}

	_, err := os.Stat(fp)
	if os.IsNotExist(err) {
		return r, err
	}

	f, e := os.Open(fp)
	if e != nil {
		return r, e
	}
	defer f.Close() //nolint:errcheck
	dec := json.NewDecoder(f)
	if err := dec.Decode(&r); err != nil {
		fmt.Printf("failure to decode config file: %v", err)
		return r, err
	}
	return r, nil
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
		fullpath = path.Clean(strings.Replace(cpath, "~", d, 1))
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
	h, err = parsePath(workdir)
	if err != nil {
		fmt.Printf("could not parse declared directory: %s\n", workdir)
		return
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

// visit is a custom function which allows all Repos to be walked across the filesystem
func visit(p string) ([]Repo, error) {
	a := make([]Repo, 0, 100)
	err := godirwalk.Walk(p, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			if strings.HasSuffix(osPathname, "/.git") {
				path := strings.TrimRight(strings.TrimPrefix(osPathname, (p+string(os.PathSeparator))), ".git")
				for _, f := range a {
					if strings.HasPrefix(path, f.Path) { // Detect if found path exists in currently scanned path
						return godirwalk.SkipThis
					}
				}
				a = append(a, Repo{Path: path})
			}
			return nil
		},
		Unsorted: false, // Setting this to true causes many problems in scanning
	})
	return a, err
}
