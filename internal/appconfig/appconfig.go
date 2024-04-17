package appconfig

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/SeungKang/blaj/internal/ini"
)

const (
	readPointerParamSuffix  = "pointer_"
	writePointerParamSuffix = "pointer"
	dataParamSuffix         = "data"
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

type General struct {
	ExeName  string
	Disabled bool
}

func (o *General) OnParam(name string) (func(param *ini.Param) error, ini.SchemaRule) {
	switch name {
	case "exename":
		return func(param *ini.Param) error {
			o.ExeName = strings.ToLower(param.Value)
			return nil
		}, ini.SchemaRule{Limit: 1}
	case "disabled":
		return func(param *ini.Param) error {
			disabled, err := strconv.ParseBool(param.Value)
			if err != nil {
				return fmt.Errorf("failed to parse boolean for disabled param - %w", err)
			}

			o.Disabled = disabled
			return nil
		}, ini.SchemaRule{Limit: 1}
	default:
		return nil, ini.SchemaRule{}
	}
}

func (o *General) Validate() error {
	//TODO: check if required params are set
	return nil
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

func (o *SaveRestore) AddParam(param *ini.Param) error {
	switch v := strings.ToLower(param.Name); v {
	case "savestate":
		saveStateKeybind, err := keybindFromStr(param.Value)
		if err != nil {
			return fmt.Errorf("failed to parse keybind: %q - %w", param.Value, err)
		}

		o.SaveState = saveStateKeybind
	case "restorestate":
		restoreStateKeybind, err := keybindFromStr(param.Value)
		if err != nil {
			return fmt.Errorf("failed to parse keybind: %q - %w", param.Value, err)
		}

		o.RestoreState = restoreStateKeybind
	default:
		if !strings.Contains(v, readPointerParamSuffix) {
			return fmt.Errorf("unknown parameter: %q", param.Name)
		}

		pointer, err := readPointerFromParam(param)
		if err != nil {
			return fmt.Errorf("failed to parse pointer: %q - %w",
				param.Name, err)
		}

		o.Pointers = append(o.Pointers, pointer)
	}

	return nil
}

func (o *SaveRestore) Validate() error {
	// TODO: validate other fields
	if len(o.Pointers) == 0 {
		return errors.New("no pointers were specified")
	}

	return nil
}

type Writer struct {
	Pointers map[string]WritePointer
	Keybind  byte

	currentWritePointer *WritePointer
}

func (o *Writer) AddParam(param *ini.Param) error {
	switch v := strings.ToLower(param.Name); v {
	case "writekeybind":
		keybind, err := keybindFromStr(param.Value)
		if err != nil {
			return fmt.Errorf("failed to parse keybind: %q - %w", param.Value, err)
		}

		o.Keybind = keybind
	default:
		if strings.Contains(param.Name, writePointerParamSuffix) {

		}

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

func (o *Writer) addWriterPointer(param *ini.Param, paramNameLC string) error {
	pointer, err := pointerFromParam(param)
	if err != nil {
		return fmt.Errorf("failed to parse pointer: %q - %w",
			param.Name, err)
	}

	name := strings.TrimSuffix(paramNameLC, writePointerParamSuffix)
	wp, _ := o.Pointers[name]
	if o.Pointers == nil {
		o.Pointers = make(map[string]WritePointer)
	}

	if wp.Pointer.Name != "" {
		return fmt.Errorf("write pointer already has a pointer defined (%q)",
			wp.Pointer.Name)
	}

	wp.Pointer = pointer
	o.Pointers[name] = wp
	return nil
}

// TODO: support spaces (strings.fields)
func (o *Writer) addData(param *ini.Param, paramNameLC string) error {
	data, err := hex.DecodeString(strings.TrimPrefix(param.Value, "0x"))
	if err != nil {
		return fmt.Errorf("failed to decode data - %w", err)
	}

	name := strings.TrimSuffix(paramNameLC, dataParamSuffix)
	wp, _ := o.Pointers[name]
	if o.Pointers == nil {
		o.Pointers = make(map[string]WritePointer)
	}

	if len(wp.Data) > 0 {
		return errors.New("write pointer already has data defined")
	}

	wp.Data = data
	o.Pointers[name] = wp
	return nil
}

//func (o *Writer) addWriterPointer(param *ini.Param) error {
//	pointer, err := pointerFromParam(param)
//	if err != nil {
//		return fmt.Errorf("failed to parse pointer: %q - %w",
//			param.Name, err)
//	}
//
//	var target *WritePointer
//	switch {
//	case o.currentWritePointer == nil:
//		o.currentWritePointer = &WritePointer{
//			Pointer: pointer,
//		}
//	case o.currentWritePointer != nil && o.currentWritePointer.Pointer.Name == "":
//		o.currentWritePointer.Pointer = pointer
//		if len(o.currentWritePointer.Data) > 0 {
//			o.Pointers = append(o.Pointers, *o.currentWritePointer)
//			o.currentWritePointer = nil
//		}
//	case o.currentWritePointer != nil && o.currentWritePointer.Pointer.Name != "":
//		return fmt.Errorf("")
//	}
//
//	o.Pointers = append(o.Pointers)
//}

func (o *Writer) Validate() error {
	//TODO implement me
	panic("implement me")
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
