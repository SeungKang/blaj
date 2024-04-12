package appconfig

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/SeungKang/blaj/internal/ini"
)

const (
	readPointerParamSuffix  = "Pointer_"
	writePointerParamSuffix = "Pointer"
)

func Parse(r io.Reader) (*Config, error) {
	iniConfig, err := ini.Parse(r)
	if err != nil {
		return nil, err
	}

	return FromINI(iniConfig)
}

func FromINI(iniConfig *ini.INI) (*Config, error) {
	var games []*Game
	for _, section := range iniConfig.Sections {
		game, err := generalSection(section)
		if err != nil {
			return nil, fmt.Errorf("failed to parse game section: %q - %w", section.Name, err)
		}

		games = append(games, game)
	}

	if len(games) == 0 {
		return nil, fmt.Errorf("no games provided")
	}

	return &Config{INI: iniConfig, Games: games}, nil
}

type Config struct {
	INI   *ini.INI
	Games []*Game
}

func generalSection(section *ini.Section, game *Game) error {
	exeName, err := section.FirstParamValue("exeName")
	if err != nil {
		return err
	}

	disabledStr, err := section.FirstParamValue("disabled")
	if err == nil {
		disabled, err := strconv.ParseBool(disabledStr)
		if err != nil {
			return fmt.Errorf("failed to parse boolean for disabled param - %w", err)
		}

		game.Disabled = disabled
	}

	game.ExeName = exeName
	return nil
}

func saveRestoreSection(section *ini.Section) (SaveRestore, error) {
	var pointers []Pointer
	for _, param := range section.Params {
		if strings.Contains(param.Name, readPointerParamSuffix) {
			pointer, err := readPointerFromParam(param)
			if err != nil {
				return SaveRestore{}, fmt.Errorf("failed to parse pointer: %q - %w",
					param.Name, err)
			}

			pointers = append(pointers, pointer)
		}
	}

	if len(pointers) == 0 {
		return SaveRestore{}, fmt.Errorf("no pointers provided")
	}

	saveStateKeybindStr, err := section.FirstParamValue("saveState")
	if err != nil {
		return SaveRestore{}, err
	}

	saveStateKeybind, err := keybindFromStr(saveStateKeybindStr)
	if err != nil {
		return SaveRestore{}, fmt.Errorf("failed to parse keybind: %q - %w", saveStateKeybindStr, err)
	}

	restoreStateKeybindStr, err := section.FirstParamValue("restoreState")
	if err != nil {
		return SaveRestore{}, err
	}

	restoreStateKeybind, err := keybindFromStr(restoreStateKeybindStr)
	if err != nil {
		return SaveRestore{}, fmt.Errorf("failed to parse keybind: %q - %w", restoreStateKeybindStr, err)
	}

	return SaveRestore{
		Pointers:     pointers,
		SaveState:    saveStateKeybind,
		RestoreState: restoreStateKeybind,
	}, nil
}

func writerSection(section *ini.Section) (Writer, error) {
	var pointers []Pointer
	for _, param := range section.Params {
		if strings.Contains(param.Name, writePointerParamSuffix) {
			pointer, err := pointerFromParam(param)
			if err != nil {
				return Writer{}, fmt.Errorf("failed to parse pointer: %q - %w",
					param.Name, err)
			}

			pointers = append(pointers, pointer)
		}
	}

	WritePointer{
		Pointer: Pointer{},
		Data:    nil,
	}

	if len(pointers) == 0 {
		return Writer{}, fmt.Errorf("no pointers provided")
	}

	keybindStr, err := section.FirstParamValue("writeKeybind")
	if err != nil {
		return Writer{}, err
	}

	keybind, err := keybindFromStr(keybindStr)
	if err != nil {
		return Writer{}, fmt.Errorf("failed to parse keybind: %q - %w", keybindStr, err)
	}

	return Writer{
		Pointers: pointers,
		Keybind:  keybind,
	}, nil
}

func readPointerFromParam(param *ini.Param) (Pointer, error) {
	if strings.Count(param.Name, readPointerParamSuffix) > 1 {
		return Pointer{}, fmt.Errorf("%q found more than once, please don't do that >:c",
			readPointerParamSuffix)
	}

	_, sizeStr, hasIt := strings.Cut(param.Name, readPointerParamSuffix)
	if !hasIt {
		return Pointer{}, fmt.Errorf("pointer missing number of bytes to save")
	}

	size, err := strconv.ParseUint(sizeStr, 10, 32)
	if err != nil {
		return Pointer{}, fmt.Errorf("failed to parse size %q - %w",
			sizeStr, err)
	}

	pointer, err := pointerFromParam(param)
	if err != nil {
		return Pointer{}, fmt.Errorf("failed to create pointer from param - %w", err)
	}

	pointer.NBytes = int(size)
	return pointer, nil
}

func pointerFromParam(param *ini.Param) (Pointer, error) {
	// TODO: support module names with spaces
	strs := strings.Fields(param.Value)
	if len(strs) == 0 {
		return Pointer{}, fmt.Errorf("pointer is empty")
	}

	var startIndex int
	var optModuleName string
	if strings.Contains(strs[0], ".") {
		startIndex = 1
		optModuleName = strs[0]
	}

	var values []uintptr
	for _, str := range strs[startIndex:] {
		str = strings.TrimPrefix(str, "0x")
		value, err := strconv.ParseUint(str, 16, 64)
		if err != nil {
			return Pointer{}, fmt.Errorf("failed to convert string to uint: %q - %w",
				str, err)
		}

		values = append(values, uintptr(value))
	}

	return Pointer{
		Name:      param.Name,
		Addrs:     values,
		OptModule: strings.ToLower(optModuleName),
	}, nil
}

func keybindFromStr(keybindStr string) (byte, error) {
	if len(keybindStr) != 1 {
		return 0, fmt.Errorf("keybind must be 1 character")
	}

	return keybindStr[0], nil
}

type Game struct {
	ExeName      string
	Disabled     bool
	SaveRestores []SaveRestore
	Writers      []Writer
}

type SaveRestore struct {
	Pointers     []Pointer
	SaveState    byte
	RestoreState byte
}

type Writer struct {
	Pointers []WritePointer
	Keybind  byte
}

type WritePointer struct {
	Pointer Pointer
	Data    []byte
}

type Pointer struct {
	Name      string
	Addrs     []uintptr
	NBytes    int
	OptModule string
}
