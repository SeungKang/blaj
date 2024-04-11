package kernel32

import (
	"fmt"
	"path/filepath"
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

type Module struct {
	Filepath string
	Filename string
	BaseAddr uintptr
}

// ProcessModules returns the target process's modules
//
// the process handle must be opened with
// windows.PROCESS_VM_READ | windows.PROCESS_QUERY_INFORMATION
func ProcessModules(processHandle syscall.Handle) ([]Module, error) {
	// TODO: handle more than 1024 (lookup maximum file handles and use that)
	moduleHandles := make([]syscall.Handle, 1024)
	numModuleHandles, err := EnumProcessModules(processHandle, moduleHandles)
	if err != nil {
		return nil, fmt.Errorf("failed to enum process modules - %w", err)
	}
	defer func() {
		for _, handle := range moduleHandles[0:numModuleHandles] {
			syscall.CloseHandle(handle)
		}
	}()

	modules := make([]Module, numModuleHandles)
	for i, moduleHandle := range moduleHandles[0:numModuleHandles] {
		module, err := lookupModuleInfo(processHandle, moduleHandle)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup module info handle: %v - %w",
				moduleHandle, err)
		}

		modules[i] = module
	}

	return modules, nil
}

func lookupModuleInfo(processHandle syscall.Handle, moduleHandle syscall.Handle) (Module, error) {
	fileName, err := GetModuleFilenameExW(processHandle, moduleHandle)
	if err != nil {
		return Module{}, fmt.Errorf("failed to get module filename - %w", err)
	}

	var info MODULEINFO
	err = GetModuleInformation(processHandle, moduleHandle, &info)
	if err != nil {
		return Module{}, fmt.Errorf("failed to get module information - %w", err)
	}

	return Module{
		Filepath: fileName,
		Filename: filepath.Base(fileName),
		BaseAddr: info.LpBaseOfDll,
	}, nil
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
