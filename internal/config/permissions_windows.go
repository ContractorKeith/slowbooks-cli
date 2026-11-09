package config

import (
	"fmt"
	"os"
	"strings"
	"unsafe"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"golang.org/x/sys/windows"
)

// Check read and permission-control grants, including inherited ACEs. Windows
// synthetic mode bits do not enforce Unix-style token privacy.
func tokenPermissions(path string, _ os.FileMode) (string, error) {
	supported, err := persistentACLs(path)
	if err != nil {
		return "", aclInspectError(path, err)
	}
	if !supported {
		return path + "; ACLs unsupported on this filesystem", nil
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return "", aclInspectError(path, err)
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil {
		return "", aclInspectError(path, fmt.Errorf("missing DACL owner"))
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return "", aclInspectError(path, err)
	}
	bad := []string{}
	if acl == nil {
		bad = append(bad, "S-1-1-0")
	} else {
		seen := map[string]bool{}
		for i := uint32(0); i < uint32(acl.AceCount); i++ {
			var ace *windows.ACCESS_ALLOWED_ACE
			if err = windows.GetAce(acl, i, &ace); err != nil {
				return "", aclInspectError(path, err)
			}
			if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
				continue
			}
			if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
				return "", aclInspectError(path, fmt.Errorf("unsupported DACL ACE type %d", ace.Header.AceType))
			}
			const unsafeMask = windows.FILE_READ_DATA | windows.GENERIC_READ | windows.GENERIC_ALL | windows.WRITE_DAC | windows.WRITE_OWNER
			if uint32(ace.Mask)&unsafeMask == 0 {
				continue
			}
			sid, err := allowedAceSID(ace)
			if err != nil {
				return "", aclInspectError(path, err)
			}
			if trustedTokenPrincipal(sid, owner) {
				continue
			}
			value := sid.String()
			if !seen[value] {
				bad = append(bad, value)
				seen[value] = true
			}
		}
	}
	if len(bad) > 0 {
		hint := `icacls "` + path + `" /inheritance:r /grant:r "*` + owner.String() + `:(F)" "*S-1-5-18:(F)" "*S-1-5-32-544:(F)" /remove:g`
		for _, sid := range bad {
			hint += ` "*` + sid + `"`
		}
		return "", &output.Error{Code: "auth", Message: "token DACL grants read or ownership/permission control to " + strings.Join(bad, ", ") + ": " + path, Hint: hint}
	}
	return path + "; DACL checked: read and permission-control grants limited to owner, SYSTEM and Administrators", nil
}

// GetAce returns native variable-length memory. Bound SID access by AceSize
// before passing the interior SID pointer back to Windows for validation.
func allowedAceSID(ace *windows.ACCESS_ALLOWED_ACE) (*windows.SID, error) {
	size := int(ace.Header.AceSize)
	if size < 16 {
		return nil, fmt.Errorf("invalid DACL ACE size")
	}
	data := unsafe.Slice((*byte)(unsafe.Pointer(ace)), size)
	if 8+8+4*int(data[9]) > size {
		return nil, fmt.Errorf("invalid DACL SID size")
	}
	sid := (*windows.SID)(unsafe.Pointer(&data[8]))
	if !sid.IsValid() {
		return nil, fmt.Errorf("invalid DACL SID")
	}
	return sid, nil
}

func aclInspectError(path string, err error) error {
	return &output.Error{Code: "auth", Message: "cannot verify token DACL: " + path + ": " + err.Error(), Hint: `inspect and restrict read access with icacls "` + path + `"`}
}

func trustedTokenPrincipal(sid, owner *windows.SID) bool {
	return sid.Equals(owner) || sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) || sid.IsWellKnown(windows.WinCreatorOwnerRightsSid) || sid.IsWellKnown(windows.WinCreatorOwnerSid)
}
