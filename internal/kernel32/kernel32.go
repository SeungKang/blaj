package kernel32

import (
	"errors"
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Various dwFilterFlag values.
//
// See also:
// https://learn.microsoft.com/en-us/windows/win32/api/psapi/nf-psapi-enumprocessmodulesex
const (
	ListModulesDefault = 0x00
	ListModules32Bit   = 0x01
	ListModules64Bit   = 0x02
	ListModulesAll     = 0x03
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	pEnumProcessModulesEx = kernel32.NewProc("K32EnumProcessModulesEx")
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
	EndAddr  uintptr
	Size     uint64
}

// ProcessModules returns the target process's modules.
//
// The process handle must be opened with:
//
//	PROCESS_VM_READ | PROCESS_QUERY_INFORMATION
func ProcessModules(processHandle syscall.Handle) ([]Module, error) {
	var modules []Module

	err := IterProcessModules(processHandle, func(i int, total uint, mod Module) error {
		if len(modules) == 0 {
			modules = make([]Module, total)
		}

		modules[i] = mod

		return nil
	})
	if err != nil {
		return nil, err
	}

	return modules, nil
}

// IterProcessModules calls iterFn for each of the target process's modules.
//
// The process handle must be opened with:
//
//	PROCESS_VM_READ | PROCESS_QUERY_INFORMATION
func IterProcessModules(processHandle syscall.Handle, iterFn func(i int, total uint, mod Module) error) error {
	totalHandles, err := TotalEnumProcessModulesEx(processHandle, ListModulesAll)
	if err != nil {
		return fmt.Errorf("failed to get total number of process handles - %w", err)
	}

	if totalHandles == 0 {
		return errors.New("total number of process handles is zero (this should never happen)")
	}

	moduleHandles := make([]syscall.Handle, totalHandles)

	actualHandlesReturned, err := EnumProcessModulesEx(
		processHandle,
		moduleHandles,
		ListModulesAll)
	if err != nil {
		return fmt.Errorf("failed to enum process modules - %w", err)
	}

	total := uint(actualHandlesReturned)

	for i, moduleHandle := range moduleHandles[0:actualHandlesReturned] {
		module, err := lookupModuleInfo(processHandle, moduleHandle)
		if err != nil {
			return fmt.Errorf("failed to lookup module info for handle: %v - %w",
				moduleHandle, err)
		}

		err = iterFn(i, total, module)
		if err != nil {
			return fmt.Errorf("failed to iterate over modules - %w", err)
		}
	}

	return nil
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
		EndAddr:  info.LpBaseOfDll + uintptr(info.SizeOfImage),
		Size:     uint64(info.SizeOfImage),
	}, nil
}

func TotalEnumProcessModulesEx(hProcess syscall.Handle, dwFilterFlag uint32) (uint32, error) {
	lpcbNeeded := uint32(0)

	_, _, err := pEnumProcessModulesEx.Call(
		uintptr(hProcess),                    // hProcess
		uintptr(0),                           // lphModule
		uintptr(0),                           // cb
		uintptr(unsafe.Pointer(&lpcbNeeded)), // lpcbNeeded
		uintptr(dwFilterFlag))
	if isEnumProcessModulesExErrFatal(err) {
		return 0, err
	}

	return lpcbNeeded / uint32(unsafe.Sizeof(syscall.Handle(0))), nil
}

func EnumProcessModulesEx(hProcess syscall.Handle, lphModule []syscall.Handle, dwFilterFlag uint32) (uint32, error) {
	lpcbNeeded := uint32(0)

	moduleArraySizeBytes := uint32(uintptr(len(lphModule)) * unsafe.Sizeof(syscall.Handle(0)))

	_, _, err := pEnumProcessModulesEx.Call(
		uintptr(hProcess),                      // hProcess
		uintptr(unsafe.Pointer(&lphModule[0])), // lphModule
		uintptr(moduleArraySizeBytes),          // cb
		uintptr(unsafe.Pointer(&lpcbNeeded)),   // lpcbNeeded
		uintptr(dwFilterFlag))
	if isEnumProcessModulesExErrFatal(err) {
		return 0, err
	}

	return lpcbNeeded / uint32(unsafe.Sizeof(syscall.Handle(0))), nil
}

func isEnumProcessModulesExErrFatal(err error) bool {
	if err == nil {
		return false
	}

	errno, isErrno := err.(syscall.Errno)
	if !isErrno {
		return true
	}

	switch errno {
	case 0:
		// OK.
	case 299:
		// Special case for:
		// Only part of a ReadProcessMemory or
		// WriteProcessMemory request was completed
		//
		// I have no idea why this is returned to us.
		// Windoze.
	default:
		return true
	}

	return false
}

func GetModuleFilenameExW(hProcess syscall.Handle, hModule syscall.Handle) (string, error) {
	lpFilename := make([]uint16, syscall.MAX_PATH)

	_, _, err := pGetModuleFileNameExW.Call(
		uintptr(hProcess),
		uintptr(hModule),
		uintptr(unsafe.Pointer(&lpFilename[0])),
		uintptr(len(lpFilename)))
	if isError(err) {
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
	if isError(err) {
		return err
	}

	return nil
}

func isError(err error) bool {
	if err == nil {
		return false
	}

	errno, isErrno := err.(syscall.Errno)
	if !isErrno {
		return true
	}

	return errno != 0
}
