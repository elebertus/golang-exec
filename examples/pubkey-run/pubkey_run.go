package main

import (
	"bytes"
	"flag"
	"log"

	"github.com/mitchellh/go-homedir"
	"github.com/stefaanc/golang-exec/runner"
	"github.com/stefaanc/golang-exec/runner/ssh"
	"github.com/stefaanc/golang-exec/script"
)

const (
	portDefault  uint   = 22
	shellDefault string = "bash"
)

var keyPath string
var user string
var host string
var port *uint
var shell string

func init() {
	keyPathDefault, _ := homedir.Expand("~/.ssh/id_rsa")
	flag.StringVar(&keyPath, "key_path", keyPathDefault, "absolute path to ssh private key")
	flag.StringVar(&user, "user", "", "user to ssh with")
	flag.StringVar(&host, "host", "", "ssh target hostname or ip")
	port = flag.Uint("port", portDefault, "ssh port")
	flag.StringVar(&shell, "shell", shellDefault, "shell used for executing the command")
	flag.Parse()
}

var ps1Script = script.New("ps_version", "powershell", ps1ScriptBody)
var ps1ScriptBody string = `
$ErrorActionPreference = 'Stop'
Get-Host | Select-Object -Property Version
`

var bashScript = script.New("bash_version", "bash", bashScriptBody)
var bashScriptBody string = `
bash --version | head -n 1 | awk '{print $4}'
`

func main() {
	c := ssh.Connection{
		Type:       "ssh",
		Host:       host,
		Port:       uint16(*port),
		User:       user,
		PubKeyPath: keyPath,
		Insecure:   true,
	}

	var stdout, stderr bytes.Buffer
	ps1, err := runner.New(c, ps1Script, "nil")
	if err != nil {
		log.Fatal(err)
	}
	ps1.SetStdoutWriter(&stdout)
	ps1.SetStderrWriter(&stderr)

	ps1Err := ps1.Run()
	if ps1Err != nil {
		log.Printf("Exit code: %d\n", ps1.ExitCode())
		log.Printf("stdout: \n%s\n", stdout.String())
		log.Printf("stderr: \n%s\n", stderr.String())
		log.Fatal(ps1Err)
	}
	log.Printf("Exit code: %d\n", ps1.ExitCode())
	log.Printf("result: \n%s\n", stdout.String())

	bash, err := runner.New(c, bashScript, "nil")
	if err != nil {
		log.Fatal(err)
	}
	var bstdout, bstderr bytes.Buffer
	bash.SetStdoutWriter(&bstdout)
	bash.SetStderrWriter(&bstderr)

	bashErr := bash.Run()
	if bashErr != nil {
		log.Printf("Exit code: %d\n", bash.ExitCode())
		log.Printf("stdout: \n%s\n", bstdout.String())
		log.Printf("stderr: \n%s\n", bstderr.String())
		log.Fatal(bashErr)
	}
	log.Printf("Exit code: %d\n", bash.ExitCode())
	log.Printf("result: \n%s\n", bstdout.String())
}
