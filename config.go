package main

type gitOptions struct {
	RepoUrl string `ini:"repourl"`
	KeyFile string `ini:"keyfile"`
	User    string `ini:"user"`
	Email   string `ini:"email"`
}

type mikrotikOptions struct {
	Host     string `ini:"host"`
	Port     uint   `ini:"port"`
	Username string `ini:"username"`
	KeyFile  string `ini:"keyfile"`
}

type configOptions struct {
	Mikrotik mikrotikOptions `ini:"mikrotik"`
	Git      gitOptions      `ini:"git"`
}
