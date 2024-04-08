package kernel32

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	pEnumProcessModules   = kernel32.NewProc("K32EnumProcessModules")
	pGetModuleFileNameExW = kernel32.NewProc("K32GetModuleFileNameExW")
	pGetModuleInformation = kernel32.NewProc("K32GetModuleInformation")
)

func IsProcess32Bit(processHandle syscall.Handle) (bool, error) {
	var isProcess32Bit bool
	err := windows.IsWow64Process(windows.Handle(processHandle), &isProcess32Bit)
	if err != nil {
		return false, fmt.Errorf("failed to check if process is 32 bit - %w", err)
	}

	return isProcess32Bit, nil
}

// ModuleBaseAddr returns the base address of the target module file
// the process handle must be opened with
// windows.PROCESS_VM_READ | windows.PROCESS_QUERY_INFORMATION
func ModuleBaseAddr(processHandle syscall.Handle, targetModuleFilename string) (uintptr, error) {
	targetModuleFilename = strings.ToLower(targetModuleFilename)

	moduleHandles := make([]syscall.Handle, 1024)
	numModuleHandles, err := EnumProcessModules(processHandle, moduleHandles)
	if err != nil {
		return 0, fmt.Errorf("failed to enum process modules - %w", err)
	}
	defer func() {
		for _, handle := range moduleHandles[0:numModuleHandles] {
			syscall.CloseHandle(handle)
		}
	}()

	// TODO: close module handle
	for _, moduleHandle := range moduleHandles[0:numModuleHandles] {
		fileName, err := GetModuleFilenameExW(processHandle, moduleHandle)
		if err != nil {
			return 0, fmt.Errorf("failed to get module filename - %w", err)
		}

		if strings.ToLower(filepath.Base(fileName)) == targetModuleFilename {
			var info MODULEINFO
			err := GetModuleInformation(processHandle, moduleHandle, &info)
			if err != nil {
				return 0, fmt.Errorf("failed to get module information - %w", err)
			}

			return info.LpBaseOfDll, nil
		}
	}

	return 0, fmt.Errorf("failed to find module filename: %s", targetModuleFilename)
}

// TODO: investigate using EnumProcessModulesEx
func EnumProcessModules(hProcess syscall.Handle, lphModule []syscall.Handle) (uintptr, error) {
	lpcbNeeded := uint32(0)

	_, _, err := pEnumProcessModules.Call(
		uintptr(hProcess),
		uintptr(unsafe.Pointer(&lphModule[0])),
		uintptr(len(lphModule)),
		uintptr(unsafe.Pointer(&lpcbNeeded)))
	if err.(syscall.Errno) != 0 {
		return 0, err
	}

	return uintptr(lpcbNeeded) / unsafe.Sizeof(syscall.Handle(0)), nil
}

func GetModuleFilenameExW(hProcess syscall.Handle, hModule syscall.Handle) (string, error) {
	lpFilename := make([]uint16, syscall.MAX_PATH)

	_, _, err := pGetModuleFileNameExW.Call(
		uintptr(hProcess),
		uintptr(hModule),
		uintptr(unsafe.Pointer(&lpFilename[0])),
		uintptr(len(lpFilename)))
	if err.(syscall.Errno) != 0 {
		return "", err
	}

	return syscall.UTF16ToString(lpFilename[:]), nil
}

type MODULEINFO struct {
	LpBaseOfDll uintptr
	SizeOfImage uint32
	EntryPoint  uintptr
}

func GetModuleInformation(hProcess syscall.Handle, hModule syscall.Handle, lpmodinfo *MODULEINFO) error {
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
