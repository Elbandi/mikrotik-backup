package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"gopkg.in/ini.v1"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

var (
	configFile string
	config     configOptions
	debug      bool
)

func RunCmd(command string) {
	cmd := exec.Command("/bin/sh")
	cmd.Stdin = strings.NewReader(command)
	err := cmd.Run()
	PrintErr(err, "Failure to exec notify")
}

func CheckErr(err error, str string) {
	if err != nil {
		if len(config.Notify.OnFailure) > 0 {
			RunCmd(config.Notify.OnFailure)
		}
		if len(config.Notify.OnFailureMsg) > 0 {
			RunCmd(config.Notify.OnFailureMsg + " " + fmt.Sprintf("Error %s: %s", str, err.Error()))
		}
		log.Fatalf("Error %s: %s", str, err.Error())
	}
}

func PrintErr(err error, str string) bool {
	if err != nil {
		log.Printf("Error %s: %s", str, err.Error())
		return true
	}
	return false
}

func DeferClose(closer io.Closer, str string) bool {
	return PrintErr(closer.Close(), str)
}

func DeferEOFClose(closer io.Closer, str string) bool {
	if err := closer.Close(); err != nil && err != io.EOF {
		return PrintErr(err, str)
	}
	return false
}

func connectToHost(config mikrotikOptions) (*gossh.Client, error) {
	auth, err := ssh.NewPublicKeysFromFile(config.Username, config.KeyFile, "")
	if err != nil {
		return nil, err
	}
	sshConfig := &gossh.ClientConfig{
		User:            config.Username,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(auth.Signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
	}
	client, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", config.Host, config.Port), sshConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func trimResponse(b []byte) string {
	return strings.TrimSpace(string(b))
}

func getSerialNumber(client *gossh.Client) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer DeferEOFClose(session, "close serialnumber session")
	serial, err := session.Output(":put [/system routerboard get serial-number]")
	if err != nil {
		return "", err
	}

	return trimResponse(serial), nil
}

func saveToFile(w *git.Worktree, filename string, source io.Reader) (written int64, err error) {
	writer, err := w.Filesystem.Create(filename)
	if err != nil {
		return
	}
	defer DeferClose(writer, "close git file")

	read := bufio.NewReader(source)
	write := bufio.NewWriter(writer)
	written = int64(0)
	for {
		str, err := read.ReadString('\n')
		if len(str) > 0 {
			if strings.HasPrefix(str, "# software id") ||
				strings.Contains(str, "by RouterOS") {
				continue
			}
			nw, err := write.WriteString(str)
			if err != nil {
				return written, err
			}
			if nw > 0 {
				written += int64(nw)
			}
		}
		if err == io.EOF {
			err := write.Flush()
			PrintErr(err, "close git file")
			break
		}
		if err != nil {
			return written, err
		}
	}
	return
}

func writeMikrotikBackup(gitConfig gitOptions, filename string, source io.Reader) error {
	fs := memfs.New()
	storer := memory.NewStorage()
	cloneOptions := git.CloneOptions{URL: gitConfig.RepoUrl}
	pushOptions := git.PushOptions{}
	if debug {
		cloneOptions.Progress = os.Stderr
		pushOptions.Progress = os.Stderr
	}
	if len(gitConfig.KeyFile) > 0 {
		auth, err := ssh.NewPublicKeysFromFile("git", gitConfig.KeyFile, "")
		if err != nil {
			log.Printf("Pubkey load error: %s", err.Error())
			return err
		}
		auth.HostKeyCallback = gossh.InsecureIgnoreHostKey()
		cloneOptions.Auth = auth
		pushOptions.Auth = auth
	}
	repo, err := git.Clone(storer, fs, &cloneOptions)
	if PrintErr(err, "Clone error") {
		return err
	}
	wt, err := repo.Worktree()
	if PrintErr(err, "Worktree error") {
		return err
	}
	err = wt.Reset(&git.ResetOptions{Mode: git.HardReset})
	if PrintErr(err, "hardreset error") {
		return err
	}
	err = wt.Checkout(&git.CheckoutOptions{})
	if PrintErr(err, "Checkout error") {
		return err
	}
	_, err = saveToFile(wt, filename, source)
	if PrintErr(err, "Save error") {
		return err
	}

	_, err = wt.Add(filename)
	if PrintErr(err, "Add error") {
		return err
	}
	status, err := wt.Status()
	if PrintErr(err, "Status error") {
		return err
	}
	if debug {
		fmt.Println("git status: ", status)
	}
	if len(status) > 0 {
		commit, err := wt.Commit(fmt.Sprintf("[Mikrotik] Auto commit for %s", filename), &git.CommitOptions{
			Author: &object.Signature{
				Name:  gitConfig.User,
				Email: gitConfig.Email,
				When:  time.Now(),
			},
		})
		if PrintErr(err, "Commit error") {
			return err
		}
		cobj, err := repo.CommitObject(commit)
		if PrintErr(err, "Commit error") {
			return err
		}
		// err = wt.Pull(&git.PullOptions{RemoteName: "origin"})
		err = repo.Push(&pushOptions)
		if PrintErr(err, "Push error") {
			return err
		}
		//		fmt.Fprintf(responseWriter, "Saved: %s", cobj.Hash.String())
		log.Printf("Saved: %s", cobj.Hash.String())
	} else {
		log.Printf("Notting to commit")
		//		fmt.Fprintf(responseWriter, "Notting to commit")
	}
	return nil
}

func usage() {
	_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", filepath.Base(os.Args[0]))
	flag.PrintDefaults()
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	flag.StringVar(&configFile, "f", "", "Location of configuration file")
	flag.BoolVar(&debug, "debug", false, "enable debug mode")
	flag.Parse()
	flag.Usage = usage

	if configFile == "" {
		flag.Usage()
		os.Exit(1)
	}
	cfg, err := ini.Load(configFile)
	CheckErr(err, "load config")

	config.Mikrotik.Port = 22 // default value
	err = cfg.MapTo(&config)
	CheckErr(err, "parse config")

	client, err := connectToHost(config.Mikrotik)
	CheckErr(err, "connect to mikrotik")
	defer DeferClose(client, "close ssh client")

	serial, err := getSerialNumber(client)
	CheckErr(err, "get serial number")
	if debug {
		fmt.Println("Mikrotik serial number: ", serial)
	}

	session, err := client.NewSession()
	CheckErr(err, "create command session")
	defer DeferEOFClose(session, "close command session")

	stdout, err := session.StdoutPipe()
	CheckErr(err, "create session pipe")

	err = session.Run("/export show-sensitive; /user export show-sensitive")
	CheckErr(err, "run export")

	err = writeMikrotikBackup(config.Git, serial, stdout)
	CheckErr(err, "write backup")

	if len(config.Notify.OnSuccess) > 0 {
		RunCmd(config.Notify.OnSuccess)
	}
}
