package appconfig

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/SeungKang/blaj/internal/ini"
)

const (
	pointerParamSuffix = "Pointer_"
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
		game, err := gameFromSection(section)
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

func gameFromSection(section *ini.Section) (*Game, error) {
	exeName, err := section.FirstParamValue("exeName")
	if err != nil {
		return nil, err
	}

	var pointers []Pointer
	for _, param := range section.Params {
		if strings.Contains(param.Name, pointerParamSuffix) {
			pointer, err := pointerFromParam(param)
			if err != nil {
				return nil, fmt.Errorf("failed to parse pointer: %q - %w",
					param.Name, err)
			}

			pointers = append(pointers, pointer)
		}
	}

	if len(pointers) == 0 {
		return nil, fmt.Errorf("no pointers provided")
	}

	saveStateKeybindStr, err := section.FirstParamValue("saveState")
	if err != nil {
		return nil, err
	}

	saveStateKeybind, err := keybindFromStr(saveStateKeybindStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse keybind: %q - %w", saveStateKeybindStr, err)
	}

	restoreStateKeybindStr, err := section.FirstParamValue("restoreState")
	if err != nil {
		return nil, err
	}

	restoreStateKeybind, err := keybindFromStr(restoreStateKeybindStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse keybind: %q - %w", restoreStateKeybindStr, err)
	}

	return &Game{
		ExeName:      exeName,
		Pointers:     pointers,
		SaveState:    saveStateKeybind,
		RestoreState: restoreStateKeybind,
	}, nil
}

func pointerFromParam(param *ini.Param) (Pointer, error) {
	if strings.Count(param.Name, pointerParamSuffix) > 1 {
		return Pointer{}, fmt.Errorf("%q found more than once, please don't do that >:c",
			pointerParamSuffix)
	}

	_, sizeStr, hasIt := strings.Cut(param.Name, pointerParamSuffix)
	if !hasIt {
		return Pointer{}, fmt.Errorf("pointer missing number of bytes to save")
	}

	size, err := strconv.ParseUint(sizeStr, 10, 32)
	if err != nil {
		return Pointer{}, fmt.Errorf("failed to parse size %q - %w",
			sizeStr, err)
	}

	strs := strings.Fields(param.Value)
	if len(strs) == 0 {
		return Pointer{}, fmt.Errorf("pointer is empty")
	}

	var values []uintptr
	for _, str := range strs {
		str = strings.TrimPrefix(str, "0x")
		value, err := strconv.ParseUint(str, 16, 64)
		if err != nil {
			return Pointer{}, fmt.Errorf("failed to convert string to uint: %q - %w",
				str, err)
		}

		values = append(values, uintptr(value))
	}

	return Pointer{
		Name:   param.Name,
		Addrs:  values,
		NBytes: int(size),
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
	Pointers     []Pointer
	SaveState    byte
	RestoreState byte
}

type Pointer struct {
	Name   string
	Addrs  []uintptr
	NBytes int
}
