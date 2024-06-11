# blaj

`blaj` is a tool to manipulate memory in a target process.
It can be used to save/restore a fixed size of memory using keybinds. It can
also write a payload to a designated memory location, activated by a keybind.

This was originally created as a programmable trainer for
speedrunning games. For example, once the desired game state is found using a tool
such as [Cheat Engine](https://www.cheatengine.org/), the user can use `blaj`
to save different parts of a game's state and then restore them (e.g. save
and restore the location of the player, so that they can teleport back to certain
area). This can help gamers practice game skips in a more consistent
and efficient manner.

## Features

- Save then restore values in target process memory
- Write payload to target process memory using pointer and offsets
- Trigger memory manipulation using keybinds (ideal for working with full screen
  applications like games)
- Minimalistic systray application featuring cute shark icons to see the status
  of `blaj` and the connected processes
- Attach to multiple processes simultaneously

## Getting Started

blaj is configured using INI configuration files store in `C:\Users\<your-user-name>\.blaj`.
This simple text-based configuration file will make it easy to define
save/restore functionality, backup, and share to others.

1. Start `blaj.exe`, this will create the configuration directory `C:\Users\<your-user-name>\.blaj`
2. Create a file ending in `.conf` (example: `mirrorsedge.conf`) in the `.blaj` directory
3. Create a `[General]` section on its own line
4. Create a `exeName` parameter and make it equal to the name of the exe of the
   target process (example: `MirrorsEdge.exe`)
5. Optional: Create a `[SaveRestore]` or `[Writer]` section on a new line and set the
   required parameters
6. Create additional configuration files within the `.blaj` directory for any
   additional target process by following the steps above

Note: The `blaj` tool must be restarted in order for changes made in a config file to take effect

## Configuration File Example: Mirror's Edge

Configuration file examples can be found in the
[examples directory](https://github.com/SeungKang/blaj/tree/main/examples).

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

The [General] section defines the `exeName` and whether the configuration file is `disabled`.
The section is required and there should be only one entry per configuration.

### `exeName`

- Type: string
- Required: Yes

The exe name of the target process which is case-insensitive.

### `disabled`

- Type: boolean (true or false)
- Required: No

Use to stop processing this config file. `blaj` will skip this exe. (Defaults
to false)

## [SaveRestore]

The [SaveRestore] section defines to save and restore chunks of memory when a keybind is activated.
For example, saving and restoring the player position in a game.
This section is optional and can have multiple entries per configuration.

### `Pointer_#`

- Type: hexadecimal space delimited
- Required: Yes

The size and the location of the memory to save/restore.

### Pointer_# Example

```
xCamPointer_4 = getGameData.dll 0x01C47590 0x70 0xF8
|__||_____| |   |_____________| |________| |_______|
|      |    |          |            |          |__ additional offsets to get to the memory location containing the value (optional)
|      |    |          |            |
|      |    |          |            |__ the offset from the module base address in hexidecimal (required)
|      |    |          |
|      |    |          |__ the module to use as the base address, default is the exeName (optional)
|      |    |
|      |    |__ indicates to save 4 bytes (32 bits) at the memory location specified by the right of the equal sign
|      |
|      |__ ending the name with "Pointer_#" lets the program know to parse the values on the right of the equal sign
| 
|___ custom name prefix
```

Can be prefixed with any name but must ends with `Pointer_#` where `#` is the number of bytes
to save/restore (e.g. `xCamPointer_4 = 0x01C47590 0x70 0xF8`). You can specify more
than one `Pointer_#` parameter in the `[SaveRestore]` section.

By default, the base address of the `exeName` will be used. To use a different
module as the base address, the module's name can be included after the equal sign.

#### Implementing a Cheat Engine pointer

Cheat Engine pointers are expressed as a base address with a series of offsets.
A pointer in the `blaj` configuration file can be defined by listing these offsets space delimited.

![cheat_engine_pointer.gif](cheat_engine_pointer.gif)

### `saveState` and `restoreState`

- Type: character
- Required: Yes

Set the keybind to save and to restore memory. (e.g. `saveState = 5` &
`restoreState = 6`) Sets the save state keybind to the keyboard key `5` and the
restore state keybind to the keyboard key '6'.

## [Writer]

The [Writer] section defines to write a payload in the target process.
For example, to set player position to a specific location.
This section is optional and can have multiple entries per configuration.

### `Pointer`

- Type: hexadecimal space delimited
- Required: Yes

The location in memory to write to. Can be prefixed with any name but must ends
with `Pointer` (e.g. `xCamPointer = 0x01C47590 0x70 0xF8`)

This is a similar structure to the `Pointer_#` parameter in the `[SaveRestore]`
section without the `_#`.

### `Payload`

- Type: hexadecimal bytes
- Required: Yes

The hexadecimal bytes to write to the memory location of the Pointer.

### `Keybind`

- Type: character
- Required: Yes

Set the keybind to write the payload to the memory location of the Pointer.
Can be assigned to a single keyboard key (e.g. `keybind = p`) sets write keybind
to the keyboard key `p`.

## Requirements

- `blaj` is made for Windows machines

## Installation

Download and run the `blaj.exe` file from the latest
[Releases](https://github.com/SeungKang/blaj/releases).

### Verification

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

## Troubleshooting

Logs are saved in the `.blaj` directory found in your home directory
