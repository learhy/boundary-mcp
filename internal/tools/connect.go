package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/learhy/boundary-mcp/internal/apibase"
	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// ConnectTools returns tools that use the boundary CLI to establish proxied
// connections and run commands on targets. These tools shell out to the
// `boundary` binary which must be on PATH.
func ConnectTools(c *bclient.Client) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{

		Registration(
			"connect_ssh",
			`Connect to an SSH target via Boundary and execute a command. Uses boundary connect to establish a proxied TCP connection, then uses sshpass+ssh to authenticate with the brokered password. Returns stdout, stderr, and exit code.`,
			ToolSchema(map[string]json.RawMessage{
				"target_id": Prop("string", "The target ID to connect to (TCP target on port 22).", nil),
				"command":   Prop("string", "Command to execute on the remote host.", nil),
				"username":  Prop("string", "SSH username (from brokered credentials).", "adminuser"),
				"password":  Prop("string", "SSH password (from brokered credentials).", nil),
				"timeout":   Prop("integer", "Timeout in seconds (default: 30).", 30),
			}, []string{"target_id", "command", "password"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					TargetID string `json:"target_id"`
					Command  string `json:"command"`
					Username string `json:"username"`
					Password string `json:"password"`
					Timeout  int    `json:"timeout"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.TargetID == "" || p.Command == "" || p.Password == "" {
					return nil, fmt.Errorf("target_id, command, and password are required")
				}
				if p.Timeout == 0 {
					p.Timeout = 30
				}

				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.Timeout+10)*time.Second)
				defer cancel()

				tokenEnv := "BOUNDARY_TOKEN=" + c.Token
				addrEnv := "BOUNDARY_ADDR=" + c.Addr

				// Use boundary connect (tcp) with sshpass+ssh as the exec binary.
				// The boundary CLI exposes BOUNDARY_PROXIED_IP/PORT to the exec'd process.
				// We write a temporary wrapper script that uses sshpass with the
				// provided password to connect via SSH through the proxy.
				username := p.Username
				if username == "" {
					username = "adminuser"
				}
				wrapperScript := "#!/bin/bash\n" +
					"HOST=$BOUNDARY_PROXIED_IP\n" +
					"PORT=$BOUNDARY_PROXIED_PORT\n" +
					"CMD=\"$1\"\n" +
					"sshpass -p '" + p.Password + "' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password -o PubkeyAuthentication=no -p $PORT " + username + "@$HOST \"$CMD\"\n"

				// Write the wrapper script to a temp file
				tmpFile, tmpErr := os.CreateTemp("", "boundary-ssh-*.sh")
				if tmpErr != nil {
					return nil, fmt.Errorf("failed to create temp script: %w", tmpErr)
				}
				tmpFile.WriteString(wrapperScript)
				tmpFile.Close()
				os.Chmod(tmpFile.Name(), 0700)
				defer os.Remove(tmpFile.Name())

				boundaryCmd := exec.CommandContext(ctx, "boundary", "connect",
					"-target-id", p.TargetID,
					"-keyring-type", "none",
					"-token", "env://BOUNDARY_TOKEN",
					"-exec", tmpFile.Name(), "--", p.Command,
				)
				boundaryCmd.Env = append(os.Environ(), addrEnv, tokenEnv)

				var stdout, stderr bytes.Buffer
				boundaryCmd.Stdout = &stdout
				boundaryCmd.Stderr = &stderr

				err := boundaryCmd.Run()
				exitCode := 0
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						exitCode = exitErr.ExitCode()
					} else {
						result := map[string]interface{}{
							"stdout":    stdout.String(),
							"stderr":    stderr.String(),
							"exit_code": -1,
							"error":     err.Error(),
						}
						out, _ := json.MarshalIndent(result, "", "  ")
						return mcp.TextResult(string(out)), nil
					}
				}

				result := map[string]interface{}{
					"stdout":    stdout.String(),
					"stderr":    stderr.String(),
					"exit_code": exitCode,
				}
				out, _ := json.MarshalIndent(result, "", "  ")
				return mcp.TextResult(string(out)), nil
			},
		),

		Registration(
			"connect_tcp",
			`Connect to a TCP target via Boundary and send a command string, then read the response. Uses boundary connect to establish a proxied TCP connection. Suitable for CLI-based devices that accept commands over TCP (e.g. router CLIs). Returns the raw response.`,
			ToolSchema(map[string]json.RawMessage{
				"target_id":       Prop("string", "The TCP target ID to connect to.", nil),
				"command":         Prop("string", "Command string to send to the target.", nil),
				"read_timeout":    Prop("integer", "How long to read response in seconds (default: 5).", 5),
				"connect_timeout": Prop("integer", "Overall timeout in seconds (default: 30).", 30),
			}, []string{"target_id", "command"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					TargetID       string `json:"target_id"`
					Command        string `json:"command"`
					ReadTimeout    int    `json:"read_timeout"`
					ConnectTimeout int    `json:"connect_timeout"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.TargetID == "" || p.Command == "" {
					return nil, fmt.Errorf("target_id and command are required")
				}
				if p.ReadTimeout == 0 {
					p.ReadTimeout = 5
				}
				if p.ConnectTimeout == 0 {
					p.ConnectTimeout = 30
				}

				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.ConnectTimeout)*time.Second)
				defer cancel()

				tokenEnv := "BOUNDARY_TOKEN=" + c.Token
				addrEnv := "BOUNDARY_ADDR=" + c.Addr

				// Send command to a TCP target via Boundary proxy.
				// boundary connect starts a local proxy and exposes the port via
				// BOUNDARY_PROXIED_HOST/BOUNDARY_PROXIED_PORT env vars to the exec'd process.
				// We use a Python script to connect to the proxy port, send the command,
				// and read the response.
				pyScript := "import socket, sys, os, time\n" +
					"host = os.environ.get('BOUNDARY_PROXIED_HOST', '127.0.0.1')\n" +
					"port = int(os.environ.get('BOUNDARY_PROXIED_PORT', '0'))\n" +
					"if port == 0:\n" +
					"    sys.stderr.write('No proxy port found')\n" +
					"    sys.exit(1)\n" +
					"cmd = sys.argv[1]\n" +
					"read_timeout = " + fmt.Sprintf("%d", p.ReadTimeout) + "\n" +
					"sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)\n" +
					"sock.settimeout(read_timeout)\n" +
					"sock.connect((host, port))\n" +
					"time.sleep(0.5)\n" +
					"try:\n" +
					"    initial = sock.recv(4096)\n" +
					"    sys.stdout.buffer.write(initial)\n" +
					"except socket.timeout:\n" +
					"    pass\n" +
					"sock.sendall((cmd + '\\r\\n').encode())\n" +
					"data = b''\n" +
					"try:\n" +
					"    while True:\n" +
					"        chunk = sock.recv(4096)\n" +
					"        if not chunk:\n" +
					"            break\n" +
					"        data += chunk\n" +
					"except socket.timeout:\n" +
					"    pass\n" +
					"sys.stdout.buffer.write(data)\n" +
					"sys.stdout.buffer.flush()\n" +
					"sock.close()\n"

				boundaryCmd := exec.CommandContext(ctx, "boundary", "connect",
					"-target-id", p.TargetID,
					"-keyring-type", "none",
					"-token", "env://BOUNDARY_TOKEN",
					"-exec", "python3", "--", "-c", pyScript, p.Command,
				)
				boundaryCmd.Env = append(os.Environ(), addrEnv, tokenEnv)

				var stdout, stderr bytes.Buffer
				boundaryCmd.Stdout = &stdout
				boundaryCmd.Stderr = &stderr

				err := boundaryCmd.Run()
				exitCode := 0
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						exitCode = exitErr.ExitCode()
					}
				}

				result := map[string]interface{}{
					"stdout":    stdout.String(),
					"stderr":    stderr.String(),
					"exit_code": exitCode,
				}
				if err != nil && exitCode == 0 {
					result["error"] = err.Error()
				}
				out, _ := json.MarshalIndent(result, "", "  ")
				return mcp.TextResult(string(out)), nil
			},
		),

		Registration(
			"connect_ssh_interactive",
			`Connect to an SSH target and run multiple commands sequentially in a single session. Each command runs and its combined output is captured. Useful for configuring network devices or running setup sequences.`,
			ToolSchema(map[string]json.RawMessage{
				"target_id": Prop("string", "The target ID.", nil),
				"commands":  ArrayProp("string", "List of commands to execute sequentially."),
				"username":  Prop("string", "SSH username.", "adminuser"),
				"password":  Prop("string", "SSH password.", nil),
				"timeout":   Prop("integer", "Timeout per command in seconds (default: 30).", 30),
			}, []string{"target_id", "commands", "password"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					TargetID string   `json:"target_id"`
					Commands []string `json:"commands"`
					Username string   `json:"username"`
					Password string   `json:"password"`
					Timeout  int      `json:"timeout"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.TargetID == "" || len(p.Commands) == 0 || p.Password == "" {
					return nil, fmt.Errorf("target_id, commands, and password are required")
				}
				if p.Timeout == 0 {
					p.Timeout = 30
				}

				joinedCmds := strings.Join(p.Commands, " && ")

				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.Timeout*len(p.Commands)+10)*time.Second)
				defer cancel()

				tokenEnv := "BOUNDARY_TOKEN=" + c.Token
				addrEnv := "BOUNDARY_ADDR=" + c.Addr

				username := p.Username
				if username == "" {
					username = "adminuser"
				}
				wrapperScript := "#!/bin/bash\n" +
					"HOST=$BOUNDARY_PROXIED_IP\n" +
					"PORT=$BOUNDARY_PROXIED_PORT\n" +
					"CMD=\"$1\"\n" +
					"sshpass -p '" + p.Password + "' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password -o PubkeyAuthentication=no -p $PORT " + username + "@$HOST \"$CMD\"\n"

				tmpFile, tmpErr := os.CreateTemp("", "boundary-ssh-*.sh")
				if tmpErr != nil {
					return nil, fmt.Errorf("failed to create temp script: %w", tmpErr)
				}
				tmpFile.WriteString(wrapperScript)
				tmpFile.Close()
				os.Chmod(tmpFile.Name(), 0700)
				defer os.Remove(tmpFile.Name())

				boundaryCmd := exec.CommandContext(ctx, "boundary", "connect",
					"-target-id", p.TargetID,
					"-keyring-type", "none",
					"-token", "env://BOUNDARY_TOKEN",
					"-exec", tmpFile.Name(), "--", joinedCmds,
				)
				boundaryCmd.Env = append(os.Environ(), addrEnv, tokenEnv)

				var stdout, stderr bytes.Buffer
				boundaryCmd.Stdout = &stdout
				boundaryCmd.Stderr = &stderr

				err := boundaryCmd.Run()
				exitCode := 0
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						exitCode = exitErr.ExitCode()
					}
				}

				result := map[string]interface{}{
					"stdout":       stdout.String(),
					"stderr":       stderr.String(),
					"exit_code":    exitCode,
					"commands_run": len(p.Commands),
				}
				out, _ := json.MarshalIndent(result, "", "  ")
				return mcp.TextResult(string(out)), nil
			},
		),
	}
}

// shellQuote quotes a string for safe use in a shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
