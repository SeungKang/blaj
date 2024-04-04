package kernel32

import (
	"fmt"
	"github.com/Andoryuuta/kiwi/w32"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	pEnumProcessModules   = kernel32.NewProc("K32EnumProcessModules")
	pGetModuleFileNameExW = kernel32.NewProc("K32GetModuleFileNameExW")
	pGetModuleInformation = kernel32.NewProc("K32GetModuleInformation")
)

// ModuleBaseAddr returns the base address of the target module file
// the process handle must be opened with
// windows.PROCESS_VM_READ | windows.PROCESS_QUERY_INFORMATION
func ModuleBaseAddr(handle w32.HANDLE, targetModuleFilename string) (uintptr, error) {
	targetModuleFilename = strings.ToLower(targetModuleFilename)

	moduleHandles, err := EnumProcessModules(handle)
	if err != nil {
		return 0, fmt.Errorf("failed to enum process modules - %w", err)
	}

	for _, moduleHandle := range moduleHandles {
		fileName, err := GetModuleFilenameExW(handle, moduleHandle)
		if err != nil {
			return 0, fmt.Errorf("failed to get module filename - %w", err)
		}

		if strings.ToLower(filepath.Base(fileName)) == targetModuleFilename {
			var info MODULEINFO
			err := GetModuleInformation(handle, moduleHandle, &info)
			if err != nil {
				return 0, fmt.Errorf("failed to get module information - %w", err)
			}

			return info.LpBaseOfDll, nil
		}
	}

	return 0, fmt.Errorf("failed to find module filename: %s", targetModuleFilename)
}

// TODO: investigate using EnumProcessModulesEx
func EnumProcessModules(hProcess w32.HANDLE) ([]w32.HANDLE, error) {
	var hMods [1024]w32.HANDLE
	needed := uint32(0)

	_, _, err := pEnumProcessModules.Call(
		uintptr(hProcess),
		uintptr(unsafe.Pointer(&hMods)),
		uintptr(len(hMods)),
		uintptr(unsafe.Pointer(&needed)))
	if err.(syscall.Errno) == 0 {
		// Number of hModules returned
		n := (uintptr(needed) / unsafe.Sizeof(w32.HANDLE(0)))
		return hMods[:n], nil
	}

	return hMods[:], err
}

func GetModuleFilenameExW(hProcess w32.HANDLE, hModule w32.HANDLE) (string, error) {
	var lpFilename [syscall.MAX_PATH]uint16

	_, _, err := pGetModuleFileNameExW.Call(
		uintptr(hProcess),
		uintptr(hModule),
		uintptr(unsafe.Pointer(&lpFilename)),
		uintptr(len(lpFilename)))
	if err.(syscall.Errno) == 0 {
		return syscall.UTF16ToString(lpFilename[:]), nil
	}

	return "", err
}

type MODULEINFO struct {
	LpBaseOfDll uintptr
	SizeOfImage uint32
	EntryPoint  uintptr
}

func GetModuleInformation(hProcess w32.HANDLE, hModule w32.HANDLE, lpmodinfo *MODULEINFO) error {
	_, _, err := pGetModuleInformation.Call(
		uintptr(hProcess),
		uintptr(hModule),
		uintptr(unsafe.Pointer(lpmodinfo)),
		uintptr(uint32(unsafe.Sizeof(*lpmodinfo))))
	if err.(syscall.Errno) != 0 {
		return err
	}

	return nil
}
