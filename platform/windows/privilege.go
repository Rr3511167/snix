//go:build windows

package windows

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	advapi32                 = windows.NewLazySystemDLL("advapi32.dll")
	procCheckTokenMembership = advapi32.NewProc("CheckTokenMembership")
)

// checkTokenMembership wraps advapi32!CheckTokenMembership. We bind it
// directly because x/sys/windows doesn't export it.
func checkTokenMembership(token windows.Token, sid *windows.SID) (bool, error) {
	var isMember int32
	r1, _, e1 := procCheckTokenMembership.Call(
		uintptr(token),
		uintptr(unsafe.Pointer(sid)),
		uintptr(unsafe.Pointer(&isMember)),
	)
	if r1 == 0 {
		return false, e1
	}
	return isMember != 0, nil
}

// ErrNotElevated is returned by IsElevated when the current process token
// does not have the Administrators group enabled. The caller should
// surface a clear message telling the user to run elevated.
var ErrNotElevated = errors.New("snix/windows: process is not running elevated (Administrator)")

// IsElevated reports whether the current process has the Administrators
// group in its effective token — i.e. WinDivertOpen will succeed (modulo
// driver state).
//
// We check via CheckTokenMembership against the well-known
// SECURITY_BUILTIN_DOMAIN_RID \ DOMAIN_ALIAS_RID_ADMINS SID. This is the
// standard Win32 pattern and works for both Run-as-admin sessions and
// services running under LocalSystem.
func IsElevated() (bool, error) {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false, err
	}
	defer windows.FreeSid(sid)

	// Passing a zero token uses the impersonation token of the calling thread.
	return checkTokenMembership(0, sid)
}
