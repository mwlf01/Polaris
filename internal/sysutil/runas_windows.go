package sysutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modWtsapi32 = syscall.NewLazyDLL("wtsapi32.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")
	modAdvapi32 = syscall.NewLazyDLL("advapi32.dll")
	modUserenv  = syscall.NewLazyDLL("userenv.dll")

	procWTSGetActiveConsoleSessionId = modKernel32.NewProc("WTSGetActiveConsoleSessionId")
	procWTSEnumerateSessionsW        = modWtsapi32.NewProc("WTSEnumerateSessionsW")
	procWTSFreeMemory                = modWtsapi32.NewProc("WTSFreeMemory")
	procWTSQueryUserToken            = modWtsapi32.NewProc("WTSQueryUserToken")
	procDuplicateTokenEx             = modAdvapi32.NewProc("DuplicateTokenEx")
	procCreateProcessAsUserW         = modAdvapi32.NewProc("CreateProcessAsUserW")
	procCreateEnvironmentBlock       = modUserenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock      = modUserenv.NewProc("DestroyEnvironmentBlock")
)

const (
	tokenPrimary          = 1
	securityImpersonation = 2
	createNoWindow        = 0x08000000
	createUnicodeEnv      = 0x00000400
	startfUseStdHandles   = 0x00000100
	wtsActive             = 0 // WTSActive session state
)

// wtsSessionInfo mirrors the native WTS_SESSION_INFOW structure.
type wtsSessionInfo struct {
	SessionID   uint32
	_           [4]byte // padding on amd64
	StationName uintptr
	State       uint32
	_           [4]byte // padding on amd64
}

// findUserToken enumerates all interactive sessions and returns a user
// token for the first one that succeeds. This handles both console and
// RDP sessions. The caller must close the returned handle.
func findUserToken() (syscall.Handle, error) {
	// 1. Try the console session first (fast path).
	consoleID, _, _ := procWTSGetActiveConsoleSessionId.Call()
	if consoleID != 0xFFFFFFFF {
		var token syscall.Handle
		r, _, _ := procWTSQueryUserToken.Call(consoleID, uintptr(unsafe.Pointer(&token)))
		if r != 0 {
			return token, nil
		}
	}

	// 2. Enumerate all sessions and try each active one.
	var pSessionInfo unsafe.Pointer
	var count uint32
	r, _, err := procWTSEnumerateSessionsW.Call(
		0, // WTS_CURRENT_SERVER_HANDLE
		0, // reserved
		1, // version
		uintptr(unsafe.Pointer(&pSessionInfo)),
		uintptr(unsafe.Pointer(&count)),
	)
	if r == 0 {
		return 0, fmt.Errorf("WTSEnumerateSessionsW failed: %w", err)
	}
	defer procWTSFreeMemory.Call(uintptr(pSessionInfo))

	entrySize := unsafe.Sizeof(wtsSessionInfo{})
	for i := uint32(0); i < count; i++ {
		info := (*wtsSessionInfo)(unsafe.Pointer(uintptr(pSessionInfo) + uintptr(i)*entrySize))
		if info.State != wtsActive {
			continue
		}
		// Skip session 0 (services).
		if info.SessionID == 0 {
			continue
		}

		var token syscall.Handle
		r, _, _ := procWTSQueryUserToken.Call(
			uintptr(info.SessionID),
			uintptr(unsafe.Pointer(&token)),
		)
		if r != 0 {
			return token, nil
		}
	}

	return 0, fmt.Errorf("no interactive user session found")
}

// RunAsLoggedInUserWithExitCode executes a command as the currently logged-in
// interactive user. The process is created with CREATE_NO_WINDOW so the user
// does not see it. stdout/stderr are captured. Returns combined output, the
// process exit code, and an error if the process could not be launched or
// exited with a non-zero code. Works for both console and RDP sessions.
func RunAsLoggedInUserWithExitCode(name string, args ...string) ([]byte, int, error) {
	if !IsSystemUser() {
		cmd := exec.Command(name, args...)
		out, err := cmd.CombinedOutput()
		exitCode := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return out, exitCode, err
	}

	// 1. Get a user token from an active session.
	userToken, err := findUserToken()
	if err != nil {
		return nil, -1, err
	}
	defer syscall.CloseHandle(userToken)

	// 2. Duplicate the token as a primary token.
	var dupToken syscall.Handle
	r, _, err := procDuplicateTokenEx.Call(
		uintptr(userToken), 0x02000000, 0,
		securityImpersonation, tokenPrimary,
		uintptr(unsafe.Pointer(&dupToken)),
	)
	if r == 0 {
		return nil, -1, fmt.Errorf("DuplicateTokenEx failed: %w", err)
	}
	defer syscall.CloseHandle(dupToken)

	// 3. Create environment block for the user.
	var envBlock uintptr
	r, _, err = procCreateEnvironmentBlock.Call(
		uintptr(unsafe.Pointer(&envBlock)), uintptr(dupToken), 0,
	)
	if r == 0 {
		return nil, -1, fmt.Errorf("CreateEnvironmentBlock failed: %w", err)
	}
	defer procDestroyEnvironmentBlock.Call(envBlock)

	// 4. Build the command line. We wrap the actual command in cmd.exe /c
	// because CreateProcessAsUserW cannot resolve app execution aliases
	// (e.g. winget.exe) directly. cmd.exe handles PATH and alias lookup.
	innerCmd := buildCommandLine(name, args)
	comspec := os.Getenv("COMSPEC")
	if comspec == "" {
		comspec = `C:\Windows\System32\cmd.exe`
	}
	cmdLine := comspec + ` /c ` + innerCmd
	cmdLinePtr, _ := syscall.UTF16PtrFromString(cmdLine)
	appName, _ := syscall.UTF16PtrFromString(comspec)

	// 5. Set up pipes for stdout/stderr capture.
	readPipe, writePipe, err := createPipe()
	if err != nil {
		return nil, -1, fmt.Errorf("creating output pipe: %w", err)
	}
	defer syscall.CloseHandle(readPipe)

	// 6. Create the process as the logged-in user.
	var si syscall.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Flags = startfUseStdHandles
	si.StdOutput = writePipe
	si.StdErr = writePipe

	var pi syscall.ProcessInformation

	r, _, err = procCreateProcessAsUserW.Call(
		uintptr(dupToken),
		uintptr(unsafe.Pointer(appName)),
		uintptr(unsafe.Pointer(cmdLinePtr)),
		0, 0, 1, // inherit handles = TRUE
		createNoWindow|createUnicodeEnv,
		envBlock, 0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	syscall.CloseHandle(writePipe)

	if r == 0 {
		return nil, -1, fmt.Errorf("CreateProcessAsUserW failed: %w", err)
	}
	defer syscall.CloseHandle(pi.Process)
	defer syscall.CloseHandle(pi.Thread)

	// 7. Read all output and wait for the process.
	output := readAllFromHandle(readPipe)

	syscall.WaitForSingleObject(pi.Process, syscall.INFINITE)

	var exitCode uint32
	syscall.GetExitCodeProcess(pi.Process, &exitCode)

	if exitCode != 0 {
		return output, int(exitCode), fmt.Errorf("exit status 0x%x", exitCode)
	}
	return output, 0, nil
}

func buildCommandLine(name string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, quote(name))
	for _, a := range args {
		parts = append(parts, quote(a))
	}
	return strings.Join(parts, " ")
}

func quote(s string) string {
	if strings.ContainsAny(s, " \t\"") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

func createPipe() (syscall.Handle, syscall.Handle, error) {
	var sa syscall.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1

	var readH, writeH syscall.Handle
	err := syscall.CreatePipe(&readH, &writeH, &sa, 0)
	if err != nil {
		return 0, 0, err
	}
	// Make the read handle non-inheritable.
	syscall.SetHandleInformation(readH, 1, 0)
	return readH, writeH, nil
}

func readAllFromHandle(h syscall.Handle) []byte {
	var buf [4096]byte
	var all []byte
	for {
		var n uint32
		err := syscall.ReadFile(h, buf[:], &n, nil)
		if n > 0 {
			all = append(all, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return all
}
