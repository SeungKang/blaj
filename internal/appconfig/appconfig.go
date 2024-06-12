package appconfig

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/SeungKang/blaj/internal/ini"
)

const (
	readPointerParamSuffix  = "pointer_"
	writePointerParamSuffix = "pointer"
	dataParamSuffix         = "data"
)

func ProgramConfigFromPath(filePath string) (*ProgramConfig, error) {
	configFile, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file - %w", err)
	}
	defer configFile.Close()

	config, err := parseProgramConfig(configFile)
	configFile.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to parse config - %w", err)
	}

	return config, nil
}

type Config struct {
	Programs []*ProgramConfig
}

func parseProgramConfig(r io.Reader) (*ProgramConfig, error) {
	programConfig := &ProgramConfig{
		Keybinds: make(map[byte][]interface{}),
	}
	err := ini.ParseSchema(r, programConfig)
	if err != nil {
		return nil, err
	}

	return programConfig, nil
}

type ProgramConfig struct {
	General      *General
	SaveRestores []*SaveRestore
	Writers      []*Writer
	Keybinds     map[byte][]interface{}
}

func (o *ProgramConfig) Rules() ini.ParserRules {
	return ini.ParserRules{
		LowercaseNames: true,
		RequiredSections: []string{
			"general",
		},
	}
}

func (o *ProgramConfig) OnGlobalParam(paramName string) (func(*ini.Param) error, ini.SchemaRule) {
	return nil, ini.SchemaRule{}
}

func (o *ProgramConfig) OnSection(name string, actualName string) (func() (ini.SectionSchema, error), ini.SchemaRule) {
	switch name {
	case "general":
		return func() (ini.SectionSchema, error) {
			o.General = &General{}

			return o.General, nil
		}, ini.SchemaRule{Limit: 1}
	case "saverestore":
		return func() (ini.SectionSchema, error) {
			saveRestore := &SaveRestore{
				config: o,
			}

			return saveRestore, nil
		}, ini.SchemaRule{}
	case "writer":
		return func() (ini.SectionSchema, error) {
			writer := &Writer{
				config: o,
			}

			return writer, nil
		}, ini.SchemaRule{}
	default:
		return nil, ini.SchemaRule{}
	}
}

type sectionIndex struct {
	index   int
	section string
}

func (o *ProgramConfig) Validate() error {
	return nil
}

type General struct {
	ExeName  string
	Disabled bool
}

func (o *General) RequiredParams() []string {
	return []string{
		"exename",
	}
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

func readPointerFromParam(param *ini.Param) (Pointer, error) {
	_, sizeStr, hasIt := strings.Cut(strings.ToLower(param.Name), readPointerParamSuffix)
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

type SaveRestore struct {
	// TODO: make Pointers into a map
	Pointers     []Pointer
	SaveState    byte
	RestoreState byte
	config       *ProgramConfig
}

func (o *SaveRestore) RequiredParams() []string {
	return []string{
		"savestate",
		"restorestate",
	}
}

func (o *SaveRestore) OnParam(name string) (func(param *ini.Param) error, ini.SchemaRule) {
	switch {
	case "savestate" == name:
		return func(param *ini.Param) error {
			saveStateKeybind, err := keybindFromStr(param.Value)
			if err != nil {
				return fmt.Errorf("failed to parse keybind: %q - %w", param.Value, err)
			}

			o.SaveState = saveStateKeybind
			return nil
		}, ini.SchemaRule{Limit: 1}
	case "restorestate" == name:
		return func(param *ini.Param) error {
			restoreStateKeybind, err := keybindFromStr(param.Value)
			if err != nil {
				return fmt.Errorf("failed to parse keybind: %q - %w", param.Value, err)
			}

			o.RestoreState = restoreStateKeybind
			return nil
		}, ini.SchemaRule{Limit: 1}
	case strings.Contains(name, readPointerParamSuffix):
		return func(param *ini.Param) error {
			pointer, err := readPointerFromParam(param)
			if err != nil {
				return fmt.Errorf("failed to parse pointer: %q - %w",
					param.Name, err)
			}

			o.Pointers = append(o.Pointers, pointer)
			return nil
		}, ini.SchemaRule{Limit: 1}
	default:
		return nil, ini.SchemaRule{}
	}
}

func (o *SaveRestore) Validate() error {
	if len(o.Pointers) == 0 {
		return errors.New("no pointers were specified")
	}

	if o.SaveState == o.RestoreState {
		return errors.New("cannot have duplicate keybind for saveState and restoreState")
	}

	for _, pointer := range o.Pointers {
		for _, saveRestore := range o.config.SaveRestores {
			for _, otherPointer := range saveRestore.Pointers {
				if pointer.Name == otherPointer.Name {
					return fmt.Errorf("%q is already declared in a previous section", pointer.Name)
				}
			}
		}
	}

	o.config.SaveRestores = append(o.config.SaveRestores, o)

	bySaveKeybinds := o.config.Keybinds[o.SaveState]
	bySaveKeybinds = append(bySaveKeybinds, o)
	o.config.Keybinds[o.SaveState] = bySaveKeybinds

	byRestoreKeybinds := o.config.Keybinds[o.RestoreState]
	byRestoreKeybinds = append(byRestoreKeybinds, o)
	o.config.Keybinds[o.RestoreState] = byRestoreKeybinds

	return nil
}

type Writer struct {
	Pointers map[string]WritePointer
	Keybind  byte
	config   *ProgramConfig
}

func (o *Writer) RequiredParams() []string {
	return []string{
		"keybind",
	}
}

func (o *Writer) OnParam(name string) (func(param *ini.Param) error, ini.SchemaRule) {
	switch {
	case "keybind" == name:
		return func(param *ini.Param) error {
			keybind, err := keybindFromStr(param.Value)
			if err != nil {
				return fmt.Errorf("failed to parse keybind: %q - %w", param.Value, err)
			}

			o.Keybind = keybind
			return nil
		}, ini.SchemaRule{Limit: 1}
	case strings.HasSuffix(name, writePointerParamSuffix):
		return func(param *ini.Param) error {

			return o.addWriterPointer(param, name)
		}, ini.SchemaRule{Limit: 1}
	case strings.HasSuffix(name, dataParamSuffix):
		return func(param *ini.Param) error {

			return o.addData(param, name)
		}, ini.SchemaRule{Limit: 1}
	default:
		return nil, ini.SchemaRule{}
	}
}

func (o *Writer) Validate() error {
	if len(o.Pointers) == 0 {
		return fmt.Errorf("no pointers provided")
	}

	for name, writePointer := range o.Pointers {
		err := writePointer.validate()
		if err != nil {
			return fmt.Errorf("failed to validate: %q - %w", name, err)
		}

		for _, writer := range o.config.Writers {
			_, hasIt := writer.Pointers[name]
			if hasIt {
				return fmt.Errorf("%q is already declared in a previous section", name)
			}
		}
	}

	o.config.Writers = append(o.config.Writers, o)

	byWriteKeybinds := o.config.Keybinds[o.Keybind]
	byWriteKeybinds = append(byWriteKeybinds, o)
	o.config.Keybinds[o.Keybind] = byWriteKeybinds

	return nil
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
	value := strings.TrimPrefix(param.Value, "0x")
	if len(value)%2 == 1 {
		value = "0" + value
	}

	data, err := hex.DecodeString(value)
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

type WritePointer struct {
	Pointer Pointer
	Data    []byte
}

func (o *WritePointer) validate() error {
	if len(o.Pointer.Addrs) == 0 {
		return errors.New("pointer not set")
	}

	if len(o.Data) == 0 {
		return fmt.Errorf("write data not provided")
	}

	return nil
}

type Pointer struct {
	Name      string
	Addrs     []uintptr
	NBytes    int
	OptModule string
}
