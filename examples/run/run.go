package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/elebertus/golang-exec/runner"
	"github.com/elebertus/golang-exec/runner/ssh"
	"github.com/elebertus/golang-exec/script"
)

func main() {
	// define connection to the server
	c := ssh.Connection{
		Type:     "ssh",
		Host:     "localhost",
		Port:     22,
		User:     "me",
		Password: "my-password",
		Insecure: true,
	}

	// create buffers to capture stdout & stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// create script runner
	wd, _ := os.Getwd()
	err := runner.Run(&c, lsScript, lsArguments{
		//        Path: wd + "\\doesn't exist",
		Path: wd,
	}, &stdout, &stderr)
	if err != nil {
		var runnerErr runner.Error
		errors.As(err, &runnerErr)
		fmt.Printf("exitcode: %d\n", runnerErr.ExitCode())
		fmt.Printf("stdout: \n%s", stdout.String())
		fmt.Printf("stderr: \n%s\n", stderr.String())
		log.Fatal(err)
	}

	// write the result
	fmt.Printf("result: \n%s", stdout.String())
}

type lsArguments struct {
	Path string
}

var lsScript = script.New("ls", "powershell", `
    $ErrorActionPreference = 'Stop'

    $dirpath = "{{.Path}}"
    Get-ChildItem -Path $dirpath | Format-Table

    exit 0
`)
