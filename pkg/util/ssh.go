package util

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"net"
	"os"
	"time"
)

const (
	timeout = 10 * time.Minute
)

func PublicKeyFile(file string) ssh.AuthMethod {
	buffer, err := Slurp(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey([]byte(*buffer))
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

func agentSigners() ([]ssh.AuthMethod, error) {
	sock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		fmt.Println("Error connecting to SSH agent falling back to key")
		keypath, err := ExpandTilde("~/.ssh/id_rsa")
		if err != nil {
			fmt.Println("Error reading id_rsa when falling back to key")
			return nil, err
		}

		return []ssh.AuthMethod{PublicKeyFile(*keypath)}, nil
	}

	agent := agent.NewClient(sock)

	signers, err := agent.Signers()
	if err != nil {
		return nil, err
	}

	return []ssh.AuthMethod{ssh.PublicKeys(signers...)}, nil
}

// SSH establishes an interactive Secure Shell session to the supplied host as user ubuntu and on port 22. SSH uses
// ssh-agent to get the key to use
func SSH(host string) error {
	port := "22"
	user := "ubuntu"

	auths, err := agentSigners()
	if err != nil {
		return err
	}
	// or get the signer from a private key file directly
	// signer, err := ssh.ParsePrivateKey(pemBytes)
	// if err != nil {
	//     log.Fatal(err)
	// }

	// get host public key
	//hostKey := getHostKey(host)

	fmt.Printf("Connecting to %s\n", host)
	cfg := ssh.ClientConfig{
		User: user,
		Auth: auths,
		// allow any host key to be used (non-prod)
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		HostKeyAlgorithms: []string{
			ssh.KeyAlgoRSA,
			ssh.KeyAlgoDSA,
			ssh.KeyAlgoECDSA256,
			ssh.KeyAlgoECDSA384,
			ssh.KeyAlgoECDSA521,
			ssh.KeyAlgoED25519,
		},
		// verify host public key
		//HostKeyCallback: ssh.FixedHostKey(hostKey),
	}

	return StartInteractiveSSHShell(&cfg, "tcp", host, port)
}

// StartInteractiveSSHShell starts an interactive SSH shell with the supplied ClientConfig
func StartInteractiveSSHShell(sshConfig *ssh.ClientConfig, network string, host string, port string) error {
	var (
		session *ssh.Session
		conn    *ssh.Client
		err     error
	)

	addr := host + ":" + port
	if conn, err = ssh.Dial(network, addr, sshConfig); err != nil {
		fmt.Printf("Failed to dial: %s", err)
		return err
	}

	encodedkey, err := Base64PrivateKey("id_rsa")
	if err != nil {
		return err
	}

	if session, err = conn.NewSession(); err != nil {
		fmt.Printf("Failed to create session: %s", err)
		return err
	}
	defer session.Close()

	if err = setupPty(session); err != nil {
		fmt.Printf("Failed to set up pseudo terminal: %s", err)
		return err
	}

	session.Stdout = os.Stdout
	session.Stdin = os.Stdin
	session.Stderr = os.Stderr

	if err = session.Setenv("BASE64_SSH_KEY", *encodedkey); err != nil {
		fmt.Printf("Failed to send SetEnv request: %s", err)
		return err
	}

	if err = session.Shell(); err != nil {
		fmt.Printf("Failed to start interactive shell: %s", err)
		return err
	}

	return session.Wait()
}

func setupPty(session *ssh.Session) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
		session.Close()
		fmt.Printf("request for pseudo terminal failed: %s", err)
		return err
	}
	return nil
}
