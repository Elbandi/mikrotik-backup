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

type notifyOptions struct {
	OnSuccess    string `ini:"onsuccess"`
	OnFailure    string `ini:"onfailure"`
	OnFailureMsg string `ini:"onfailure_msg"`
}

type configOptions struct {
	Mikrotik mikrotikOptions `ini:"mikrotik"`
	Notify   notifyOptions   `ini:"notify,omitempty"`
	Git      gitOptions      `ini:"git"`
}
