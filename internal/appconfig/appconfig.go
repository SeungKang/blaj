package appconfig

import (
	"fmt"
	"github.com/SeungKang/speedometer/internal/ini"
	"io"
	"strconv"
	"strings"
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
		if strings.HasSuffix(param.Name, "Pointer") {
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

	setTeleportKeybindStr, err := section.FirstParamValue("setTeleport")
	if err != nil {
		return nil, err
	}

	setTeleportKeybind, err := keybindFromStr(setTeleportKeybindStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse keybind: %q - %w", setTeleportKeybindStr, err)
	}

	doTeleportKeybindStr, err := section.FirstParamValue("doTeleport")
	if err != nil {
		return nil, err
	}

	doTeleportKeybind, err := keybindFromStr(doTeleportKeybindStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse keybind: %q - %w", doTeleportKeybindStr, err)
	}

	return &Game{
		ExeName:     exeName,
		Pointers:    pointers,
		SetTeleport: setTeleportKeybind,
		DoTeleport:  doTeleportKeybind,
	}, nil
}

func pointerFromParam(param *ini.Param) (Pointer, error) {
	strs := strings.Fields(param.Value)
	if len(strs) == 0 {
		return Pointer{}, fmt.Errorf("pointer is empty")
	}

	var values []uint32
	for _, str := range strs {
		str = strings.TrimPrefix(str, "0x")
		value, err := strconv.ParseUint(str, 16, 32)
		if err != nil {
			return Pointer{}, fmt.Errorf("failed to convert string to uint: %q - %w",
				str, err)
		}

		values = append(values, uint32(value))
	}

	return Pointer{Addrs: values}, nil
}

func keybindFromStr(keybindStr string) (uint32, error) {
	value, err := strconv.ParseUint(keybindStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to convert string to uint: %q - %w",
			keybindStr, err)
	}

	return uint32(value), nil
}

type Game struct {
	ExeName     string
	Pointers    []Pointer
	SetTeleport uint32 // support string like "w"
	DoTeleport  uint32
}

type Pointer struct {
	Addrs []uint32
}
