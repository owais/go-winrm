package winrm

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf16"
)

// PowerShellSingleQuotedStringLiteral returns a string that is a valid single quoted string literal in PowerShell, that evaluates to the
// value of v.
func PowerShellSingleQuotedStringLiteral(v string) string {
	var sb strings.Builder
	_, _ = sb.WriteRune('\'')
	for _, rune := range v {
		var esc string
		switch rune {
		case '\n':
			esc = "`n"
		case '\r':
			esc = "`r"
		case '\t':
			esc = "`t"
		case '\a':
			esc = "`a"
		case '\b':
			esc = "`b"
		case '\f':
			esc = "`f"
		case '\v':
			esc = "`v"
		case '"':
			esc = "`\""
		case '\'':
			esc = "`'"
		case '`':
			esc = "``"
		case '\x00':
			esc = "`0"
		default:
			_, _ = sb.WriteRune(rune)
			continue
		}
		_, _ = sb.WriteString(esc)
	}
	_, _ = sb.WriteRune('\'')
	return sb.String()
}

// FormatPowerShellScriptCommandLine returns the command and arguments to run the specified PowerShell script.
// The returned slice contains the following elements:
// PowerShell -NoProfile -NonInteractive -ExecutionPolicy Unrestricted -EncodedCommand <base64>
func FormatPowerShellScriptCommandLine(script string) []string {
	var scriptUTF16LE bytes.Buffer
	for _, rune := range script {
		r1, r2 := utf16.EncodeRune(rune)
		if r2 == unicode.ReplacementChar {
			if rune > unicode.MaxRune {
				panic("script is not valid UTF-8")
			}
			var codeUnit [2]byte
			binary.LittleEndian.PutUint16(codeUnit[:], uint16(rune))
			_, _ = scriptUTF16LE.Write(codeUnit[:])
		} else {
			var codePoint [4]byte
			scriptUTF16LE.Grow(4)
			binary.LittleEndian.PutUint16(codePoint[0:2], uint16(r1))
			binary.LittleEndian.PutUint16(codePoint[2:4], uint16(r2))
			_, _ = scriptUTF16LE.Write(codePoint[:])
		}
	}
	scriptEncoded := base64.StdEncoding.EncodeToString(scriptUTF16LE.Bytes())
	return []string{
		"PowerShell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Unrestricted", "-EncodedCommand", scriptEncoded,
	}
}

// RunCommand is a safe utility that runs a command on the supplied shell. It copies the remote command's stderr and stdout to os.Stderr
// and os.Stdout, respectively. It also waits for the command to complete and then signals the command in case it does not terminate by
// itself, to avoid leaking resources. Use (*Shell).StartCommand for a lower level alternative.
// winrsConsoleModeStdin and winrsSkipCmdShell correspond to the SOAP options WINRS_CONSOLEMODE_STDIN and WINRS_SKIP_CMD_SHELL, respectively,
// and are defined here: https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-wsmv/c793e333-c409-43c6-a2eb-6ae2489c7ef4
func RunCommand(shell *Shell, command string, args []string, winrsConsoleModeStdin, winrsSkipCmdShell bool) error {
	stdout := bytes.NewBuffer([]byte{})
	stderr := bytes.NewBuffer([]byte{})
	return RunCommandWithOutput(shell, command, args, winrsConsoleModeStdin, winrsSkipCmdShell, stdout, stderr)
}

func RunCommandWithOutput(shell *Shell, command string, args []string, winrsConsoleModeStdin, winrsSkipCmdShell bool, stdout, stderr io.ReadWriter) error {
	if shell == nil {
		return fmt.Errorf("shell cannot be nil")
	}
	cmd, err := shell.StartCommand(command, args, winrsConsoleModeStdin, winrsSkipCmdShell, stdout, stderr)
	if err != nil {
		return err
	}
	defer cmd.Signal()
	return cmd.Wait()
}

// MustRunCommand wraps a call to RunCommand. If RunCommand returns an error then MustRunCommand panics.
func MustRunCommand(shell *Shell, command string, args []string, winrsConsoleModeStdin, winrsSkipCmdShell bool) {
	err := RunCommand(shell, command, args, winrsConsoleModeStdin, winrsSkipCmdShell)
	if err != nil {
		panic(fmt.Errorf("error while running command %+v: %v", append([]string{command}, args...), err))
	}
}

func formatBytes(bytes float64) string {
	units := []string{
		"bytes",
		"KiB",
		"MiB",
		"GiB",
	}
	logBase1024 := 0
	for bytes > 1024.0 && logBase1024 < len(units) {
		bytes /= 1024.0
		logBase1024++
	}
	return fmt.Sprintf("%.3f %s", bytes, units[logBase1024])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FormatURL formats the HTTP URL of a WinRM endpoint.
func FormatURL(useTLS bool, host string, port int) string {
	scheme := "http"
	if useTLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d/wsman", scheme, host, port)
}
