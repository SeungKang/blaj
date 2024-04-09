                                  

# blaj

blaj is a tool for parsing a config file. It lets the user set pointers and sizes
to save and restore process memory using keybinds on Windows machines.

This tool is useful for making trainers for speedrunning games. It lets users
save different parts of a game's state and then restore them, which helps in
practicing game skips effectively.

## Features

- Create conf file to specify the game, memory addresses, and keybinds.
- Creates a systray icon to manage the app and monitor game status, featuring cute shark icons.
- Customize keybinds for saving and restoring state.
- Attach the app to multiple games simultaneously.

## INI Config

```
[Mirrors Edge]
exeName = MirrorsEdge.exe
positionPointer_12 = 0x01C553D0 0xCC 0x1CC 0x2F8 0xE8
averageSpeedPointer_4 = 0x01C553D0 0xCC 0x1CC 0x2F8 0x4B8
xCamPointer_4 = 0x01C47590 0x70 0xF8
yCamPointer_4 = 0x01C47590 0x70 0xF4
saveState = 4
restoreState = 5
```

- Save config file as config.conf in .blaj file in the home directory: `%HOME%/.blaj/config.conf`
- Lines can be commented out with a # symbol
- Each section is in brackets [] and can be any name you like and does not need to be unique

### exeName

  - parameter name cannot be changed
  - value is case insensitive

### Pointer_#

  - Pointers can be prefixed with any name but must ends with `Pointer_#` 
    where # is the number of bytes to save/restore
  - values are in base 16

### saveState and restoreState

  - the parameter name cannot be changed
  - can be assigned to a single keyboard key
