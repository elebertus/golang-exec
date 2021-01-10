//
// Copyright (c) 2019 Stefaan Coussement
// MIT License
//
// more info: https://github.com/elebertus/golang-exec
//
package ssh

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/elebertus/golang-exec/script"
)

//------------------------------------------------------------------------------

type Connection struct {
	Type       string // must be "ssh"
	Host       string
	Port       uint16
	User       string
	Password   string
	PubKeyPath string
	PubKey     ssh.AuthMethod
	Insecure   bool
}

type Error struct {
	script   *script.Script
	command  string
	exitCode int
	err      error
}

type Runner struct {
	script  *script.Script
	command string
	client  *ssh.Client
	session *ssh.Session
	running bool

	exitCode int
}

//------------------------------------------------------------------------------

func (e *Error) Script() *script.Script { return e.script }
func (e *Error) Command() string        { return e.command }
func (e *Error) ExitCode() int          { return e.exitCode }
func (e *Error) Error() string          { return e.err.Error() }
func (e *Error) Unwrap() error          { return e.err }

//------------------------------------------------------------------------------

func New(connection interface{}, s *script.Script, arguments interface{}) (*Runner, error) {
	if s.Error != nil {
		return nil, &Error{
			script:   s,
			exitCode: -1,
			err:      fmt.Errorf("[golang-exec/runner/ssh/New()] script failed to parse: %#w\n", s.Error),
		}
	}

	c := toConnection(connection)
	r := new(Runner)
	r.script = s
	r.command = s.Command()

	stdin, err := s.NewReader(arguments)
	if err != nil {
		return nil, &Error{
			script:   s,
			exitCode: -1,
			err:      fmt.Errorf("[golang-exec/runner/ssh/New()] cannot create stdin reader: %#w\n", err),
		}
	}

	address := fmt.Sprintf("%s:%d", c.Host, c.Port)
	var authMethods []ssh.AuthMethod
	if len(c.Password) > 0 && c.PubKey == nil {
		authMethods = append(authMethods, ssh.Password(c.Password))
	} else {
		authMethods = append(authMethods, c.PubKey)
	}
	config := &ssh.ClientConfig{
		User: c.User,
		Auth: authMethods,
	}
	if c.Insecure {
		config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		f, err := homedir.Expand("~/.ssh/known_hosts")
		if err != nil {
			return nil, &Error{
				script:   s,
				exitCode: -1,
				err:      fmt.Errorf("[golang-exec/runner/ssh/New()] cannot find home directory of current user: %#w\n", err),
			}
		}

		hostKeyCallback, err := knownhosts.New(f)
		if err != nil {
			return nil, &Error{
				script:   s,
				exitCode: -1,
				err:      fmt.Errorf("[golang-exec/runner/ssh/New()] cannot access 'known_hosts'-file: %#w\n", err),
			}
		}
		config.HostKeyCallback = hostKeyCallback
	}

	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, &Error{
			script:   s,
			exitCode: -1,
			err:      fmt.Errorf("[golang-exec/runner/ssh/New()] cannot dial host: %#w\n", err),
		}
	}
	r.client = client

	session, err := client.NewSession()
	if err != nil {
		return nil, &Error{
			script:   s,
			exitCode: -1,
			err:      fmt.Errorf("[golang-exec/runner/ssh/New()] cannot open session: %#w\n", err),
		}
	}
	r.session = session
	r.session.Stdin = stdin

	return r, nil
}

func toConnection(connection interface{}) *Connection {
	c := new(Connection)

	v := reflect.Indirect(reflect.ValueOf(connection))
	if v.Kind() == reflect.Struct {
		c.Type = v.FieldByName("Type").String()
		c.Host = v.FieldByName("Host").String()
		c.Port = uint16(v.FieldByName("Port").Uint())
		c.User = v.FieldByName("User").String()
		c.Password = v.FieldByName("Password").String()
		c.Insecure = v.FieldByName("Insecure").Bool()
		c.PubKeyPath = v.FieldByName("PubKeyPath").String()
		if len(c.PubKeyPath) > 0 {
			_ = c.loadPubKey(c.PubKeyPath)
			// if err != nil {
			// TODO @elebertus same thing here, not sure how best to handle this without refactoring
			// can't continue because not in a loop, so just throwing the error away
			// }
		}
	} else if v.Kind() == reflect.Map {
		iter := v.MapRange()
		for iter.Next() {
			switch iter.Key().String() {
			case "Type":
				c.Type = iter.Value().String()
			case "Host":
				c.Host = iter.Value().String()
			case "Port":
				p, err := strconv.ParseUint(iter.Value().String(), 10, 16)
				if err != nil {
					p = 0
				}
				c.Port = uint16(p)
			case "User":
				c.User = iter.Value().String()
			case "Password":
				c.Password = iter.Value().String()
			case "Insecure":
				b, err := strconv.ParseBool(strings.ToLower(iter.Value().String()))
				if err != nil {
					b = false
				}
				c.Insecure = b
			case "PubKeyPath":
				c.PubKeyPath = iter.Value().String()
				err := c.loadPubKey(c.PubKeyPath)
				if err != nil {
					// TODO @elebertus maybe we shouldn't be trying to load this here? Either way
					// if the file or path is invalid errors would be raised. This function just doesn't
					// handle that though?
					continue
				}
			}
		}

	}

	return c
}

func (c *Connection) loadPubKey(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		fmt.Println(err)
		return err
	}
	kf, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return err
	}
	// @elebertus note that this doesn't support password protected keys
	// while crypto/ssh supports that, no need for it right now.
	sig, err := ssh.ParsePrivateKey(kf)
	if err != nil {
		fmt.Println(err)
		return err
	}
	c.PubKey = ssh.PublicKeys(sig)
	return nil
}

//------------------------------------------------------------------------------

func (r *Runner) SetStdoutWriter(stdout io.Writer) {
	r.session.Stdout = stdout
}

func (r *Runner) SetStderrWriter(stderr io.Writer) {
	r.session.Stderr = stderr
}

func (r *Runner) StdoutPipe() (io.Reader, error) {
	reader, err := r.session.StdoutPipe()
	if err != nil {
		r.exitCode = -1
		return nil, &Error{
			script:   r.script,
			exitCode: r.exitCode,
			err:      fmt.Errorf("[golang-exec/runner/ssh/StdoutPipe()] cannot create stdout reader: %#w\n", err),
		}
	}

	return reader, nil
}

func (r *Runner) StderrPipe() (io.Reader, error) {
	reader, err := r.session.StderrPipe()
	if err != nil {
		r.exitCode = -1
		return nil, &Error{
			script:   r.script,
			exitCode: r.exitCode,
			err:      fmt.Errorf("[golang-exec/runner/ssh/StderrPipe()] cannot create stderr reader: %#w\n", err),
		}
	}

	return reader, nil
}

func (r *Runner) Run() error {
	err := r.session.Run(r.command)
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			r.exitCode = exitErr.Waitmsg.ExitStatus()
			return &Error{
				script:   r.script,
				command:  r.command,
				exitCode: r.exitCode,
				err:      fmt.Errorf("[golang-exec/runner/ssh/Run()] runner failed: %#w\n", err),
			}
		} else {
			r.exitCode = -1
			return &Error{
				script:   r.script,
				command:  r.command,
				exitCode: r.exitCode,
				err:      fmt.Errorf("[golang-exec/runner/ssh/Run()] cannot execute runner: %#w\n", err),
			}
		}
	}

	r.exitCode = 0
	return nil
}

func (r *Runner) Start() error {
	err := r.session.Start(r.command)
	if err != nil {
		r.exitCode = -1
		return &Error{
			script:   r.script,
			command:  r.command,
			exitCode: r.exitCode,
			err:      fmt.Errorf("[golang-exec/runner/ssh/Start()] cannot start runner: %#w\n", err),
		}
	}
	r.running = true

	return nil
}

func (r *Runner) Wait() error {
	err := r.session.Wait()
	r.running = false
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			r.exitCode = exitErr.Waitmsg.ExitStatus()
		} else {
			r.exitCode = -1
		}
		return &Error{
			script:   r.script,
			command:  r.command,
			exitCode: r.exitCode,
			err:      fmt.Errorf("[golang-exec/runner/ssh/Wait()] runner failed: %#w\n", err),
		}
	}

	r.exitCode = 0
	return nil
}

func (r *Runner) Close() error {
	if r.running {
		_ = r.session.Signal(ssh.SIGTERM)
	}

	if r.session != nil {
		_ = r.session.Close()
	}

	if r.client != nil {
		r.client.Close()
	}

	return nil
}

func (r *Runner) ExitCode() int {
	return r.exitCode
}

//------------------------------------------------------------------------------
