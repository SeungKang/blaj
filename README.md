# blaj

blaj is a tool to manipulate memory in a target-process.
It can be used to save/restore a fixed size of memory using keybinds. It can
also write a payload to a designated memory location, activated by a keybind.
The information needed to perform these actions are detailed in an INI config file.
This simple text-based configuration file will make it easy to define
save/restore functionality, backup, and share to others.

A usecase for this tool is that it can be used to create a simple universal trainer for
speedrunning games. Once the desired pointers in a game are found using a tool
such as [Cheat Engine](https://www.cheatengine.org/), the user could use `blaj`
to save different parts of a game's state and then restore them (e.g. save
and restore the location of the player, so that they can teleport back to certain
area). This can help gamers practice game skips in a more consistent
and efficient manner.

## Features

- Save then restore values in target-process memory
- Write payload to target-process memory using pointer and offsets
- Create conf file to specify the process, memory addresses, and set keybinds
- Creates a systray icon to see the status of `blaj` and the connected processes
- Systray features cute shark status icons which bring joy
- Attach to multiple processes simultaneously
- Logs are saved in the `.blaj` directory found in your home directory

## Creating the config file

1. Create a directory called `.blah` in your home directory (usually
   `C:\Users\yourusername`)
3. In the `.blaj` directory, create a `.conf` file (example: `mirrorsedge.conf`)
4. Create a `[General]` section on its own line
5. Create a `exeName` parameter and make it equal to the name of the exe of the
   target-process (example: `MirrorsEdge.exe`)
6. Optional: Create a `[SaveRestore]` section on a new line and set the
   required paramters
8. Optional: Create a `[Writer]` section on a new line and set the
   required parameters
9. Create additional configuration files within the `.blaj` directory for any
   additional `target-process` by following the steps above
10. The `blaj` tool must be restarted in order for changes made in a config file
    to take effect

Several examples of configuration files can be found in the [examples directory](https://github.com/SeungKang/blaj/tree/main/examples). 

## Configuration File Example: Mirror's Edge

Section names are enclosed in square brackets and must begin at the beginning
of a line (e.g. `[General]`, `[SaveRestore]`). Data as key-value pairs are
demarcated with an equals sign (e.g. `exeName = MirrorsEdge.exe`).

```ini
# this is an example comment that blaj ignores :)

[General]
exeName = MirrorsEdge.exe
disabled = false

[SaveRestore]
positionPointer_12 = 0x01C553D0 0xCC 0x1CC 0x2F8 0xE8
averageSpeedPointer_4 = 0x01C553D0 0xCC 0x1CC 0x2F8 0x4B8
xCamPointer_4 = 0x01C47590 0x70 0xF8
yCamPointer_4 = 0x01C47590 0x70 0xF4
saveState = 4
restoreState = 5

[Writer]
positionPointer = 0x01C553D0 0xCC 0x1CC 0x2F8 0xE8
write = 00000000
keybind = 5
```

# Configuration Sections
- **[[General]](https://github.com/SeungKang/blaj?tab=readme-ov-file#general)**
- **[[SaveRestore]](https://github.com/SeungKang/blaj?tab=readme-ov-file#saverestore)**
- **[[Writer]](https://github.com/SeungKang/blaj?tab=readme-ov-file#writer)**

## [General]

The [General] section is required and there should be only one entry per configuration.

### `exeName` (Required, string)

The exe name of the target-process which is case insensitive.

### `disabled` (Optional, boolean)

Use to stop processing this config file. `blaj` will skip this exe. (Defaults
to false)

## [SaveRestore]

The [SaveRestore] section is optional and can have multiple entries per configuration.

### `Pointer_#` (Required, hexidecimal space delimited)

The size and the location of the memory to save/restore. Can be prefixed
with any name but must ends with `Pointer_#` where `#` is the number of bytes
to save/restore (e.g. `xCamPointer_4 = 0x01C47590 0x70 0xF8`). You can specify more
than one `Pointer_#` parameter in the `[SaveRestore]` section.

Optional: Indicate the module to use as the base address of the pointer, otherwise
the base address of the `exeName` will be used.

### Pointer_# structure

```
xCamPointer_4 = getGameData.dll 0x01C47590 0x70 0xF8
|__||_____| |   |_____________| |________| |_______|
|      |    |          |            |          |__ additional offsets to get to the memory location containing the value (optional)
|      |    |          |            |
|      |    |          |            |__ the offset from the module base address in hexidecimal (required)
|      |    |          |
|      |    |          |__ the module to use as the base address (optional)
|      |    |
|      |    |__ indicates to save 4 bytes (32 bits) at the memory location specified by the right of the equal sign
|      |
|      |__ ending the name with "Pointer_#" lets the program know to parse the values on the right of the equal sign
| 
|___ custom name prefix
```

### `saveState` and `restoreState` (Required, character)

Set the keybind to save and to restore memory. (e.g. `saveState = 5` &
`restoreState = 6`) Sets the save state keybind to the keyboard key `5` and the
restore state keybind to the keyboard key '6'.

## [Writer]

The [Writer] section is optional and can have multiple entries per configuration.

### `Pointer` (Required, hexidecimal space delimited)

The location in memory to write to. Can be prefixed with any name but must ends
with `Pointer` (e.g. `xCamPointer = 0x01C47590 0x70 0xF8`)

Note that this is identical to the `Pointer_#` parameter in the `[SaveRestore]`
section without the `_#`, since size is not required.

### `Payload` (Required, hexidecimal bytes)

The hexidecimal bytes to write to the memory location of the Pointer.

### `Keybind` (Required)

Set the keybind to write the payload to the memory location of the Pointer.
Can be assigned to a single keyboard key (e.g. `keybind = p`) sets write keybind
to the keyboard key `p`.

## Requirements

- `blaj` is made for Windows machines

## Installation

Download the `blaj.exe` file from the latest [Releases](https://github.com/SeungKang/blaj/releases).

### Verify the exe

The executable file found under Releases was signed using Sigstore's `cosign` tool.
You can use `cosign` to verify the file's provenance, confirming it was built by
GitHub Actions and hasn't been tampered with. Receiving a "Verified OK" output
provides a cryptographic attestation that this file came from GitHub Actions.

1. Install cosign https://docs.sigstore.dev/system_config/installation/
2. Download `blaj.exe` and `cosign.bundle` from Releases
3. Run the command below to verify

```console
# replace NAME-OF-RELEASE with the release # of the exe
$ cosign verify-blob path/to/blaj.exe \
  --bundle path/to/cosign.bundle \
  --certificate-identity=https://github.com/SeungKang/blaj/.github/workflows/build.yaml@refs/tags/NAME-OF-RELEASE \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
```

When it completes you should receive the following output:

```console
Verified OK
```
