![shark_blue.ico](icons/shark_blue.ico)

# blaj

`blaj` is a tool for creating a programmable trainer for speedrunning games.
It can be used to save/restore a chunk of memory using keybinds. It can also
write data to a designated memory location, activated by a keybind.

Once the desired game state is found using a tool such as [Cheat Engine][cheat-engine],
the user can use `blaj` to save different parts of a game's state and then
restore them (e.g. save and restore the location of the player, so that they can
teleport back to certain area). This can help gamers practice game skips in
a more consistent and efficient manner.

[cheat-engine]: https://www.cheatengine.org/

## Features

- Save and restore values in target process memory
- Write payload to target process memory using pointer and offsets
- Trigger memory manipulation using keybinds (ideal for working with full screen
  applications like games)
- Minimalistic systray application featuring cute shark icons to see the status
  of `blaj` and the connected processes
- Attach to multiple processes simultaneously

## Requirements

- `blaj` is made for Windows machines

## Installation

Download and run the `blaj.exe` file from the latest
[Releases](https://github.com/SeungKang/blaj/releases).

You may need to create a Windows Defender exclusion for the `blaj.exe` file.
Otherwise, Windows Defender will probably flag and delete it. Refer to the
[Windows documentation][windows-exclusion] for more information.

[windows-exclusion]: https://support.microsoft.com/en-us/windows/add-an-exclusion-to-windows-security-811816c0-4dfd-af4a-47e4-c301afe13b26

### Verification

The executable file found under [Releases][releases] was signed using Sigstore's `cosign`
tool. You can use `cosign` to verify the file's provenance, confirming it was
built by GitHub Actions and hasn't been tampered with. Receiving a "Verified OK"
output provides a cryptographic attestation that this file came from GitHub
Actions.

[releases]: https://github.com/SeungKang/blaj/releases

1. Install cosign (https://docs.sigstore.dev/system_config/installation/)
2. Download `blaj.exe` and `cosign.bundle` from Releases
3. Run the command below to verify. Note: Replace NAME-OF-RELEASE with the release # from GitHub.

```sh
cosign verify-blob path/to/blaj.exe \
  --bundle path/to/cosign.bundle \
  --certificate-identity=https://github.com/SeungKang/blaj/.github/workflows/build.yaml@refs/tags/NAME-OF-RELEASE \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

When it completes you should receive the following output:

```console
Verified OK
```

## Getting Started

`blaj` is configured using INI configuration files stored in
`C:\Users\<your-user-name>\.blaj`. This simple text-based configuration file
format makes it easy to define functionality, backup, and share with others.

1. Start `blaj.exe`, this will create the configuration directory at
   `C:\Users\<your-user-name>\.blaj`
2. Create a file ending in `.conf` (example: `mirrors-edge.conf`) in the `.blaj`
   directory
3. Create a `[General]` section on its own line
4. Create a `exeName` parameter and make it equal to the name of the exe of the
   target process (example: `MirrorsEdge.exe`)
5. Optional: Create a `[SaveRestore]` or `[Writer]` section on a new line
   and set the required parameters
6. Create additional configuration files within the `.blaj` directory for any
   additional target process by following the steps above

Note: The `blaj` tool must be restarted in order for changes made in a config
file to take effect

## Configuration File Example: Mirror's Edge

More configuration file examples can be found in the [examples directory](examples).

```ini
# this is an example comment that blaj ignores :)

[General]
exeName = MirrorsEdge.exe
disabled = false

[SaveRestore]
xCoordPointer_4 = 0x01C553D0 0xCC 0x1CC 0x2F8 0xE8
yCoordPointer_4 = 0x01C553D0 0xCC 0x1CC 0x2F8 0xEC
zCoordPointer_4 = 0x01C553D0 0xCC 0x1CC 0x2F8 0xF0
saveState = 4
restoreState = 5

[Writer]
bagCountPointer = 0x01C55EA8 0x194 0x128 0x3C 0x11C 0x64 0x4C 0x7A4
bagCountData = 0x00000000
keybind = 6
```

## Configuration Syntax

The following subsections document the configuration file syntax.

## `[General]`

The [General] section defines the `exeName` and whether the configuration file
is `disabled`. The section is required and there should be only one entry per
configuration.

### `exeName`

- Type: string
- Required: Yes

The exe name of the target process (case-insensitive).

### `disabled`

- Type: boolean (true or false)
- Required: No

Set to `true` to skip this config file (Defaults to false)

## `[SaveRestore]`

The [SaveRestore] section defines to save and restore chunks of memory when
a keybind is activated. For example, saving and restoring the player position
in a game. This section is optional and can have multiple entries per
configuration.

### `<nickname>Pointer_#`

- Type: hexadecimal space delimited
- Required: Yes

The size and the location of the memory to save and restore.

Here is an example:

```ini
xCamPointer_4 = getGameData.dll 0x01C47590 0x70 0xF8
#__||_____| |   |_____________| |________| |_______|
# |     |   |          |            |          |__ additional offsets to get to
# |     |   |          |            |              the memory location containing
# |     |   |          |            |              the value (optional)
# |     |   |          |            |
# |     |   |          |            |__ the offset from the module
# |     |   |          |                base address in hexidecimal
# |     |   |          |                (required)
# |     |   |          |
# |     |   |          |__ the module to use as the
# |     |   |              base address, default is
# |     |   |              the exeName (optional)
# |     |   |
# |     |   |__ memory chunk size. this example
# |     |       saves 4 bytes (32 bits) at the
# |     |       memory location specified by the
# |     |       right of the equals sign (required)
# |     |
# |     |__ required parameter suffix
# |         identifying that this is
# |         a Pointer parameter (required)
# |
# |___ custom name prefix (required)
```

Can be prefixed with any name, but must end with `Pointer_#` where `#` is the
number of bytes to save/restore (e.g. `xCamPointer_4`). You can specify multiple
`<nickname>Pointer_#` parameters in the `[SaveRestore]` section.

By default, the base address of the `exeName` will be used. To use a different
module as the base address, the module's name can be included after the equals
sign.

#### Implementing a Cheat Engine pointer

Cheat Engine pointers are expressed as a base address with a series of offsets.
A pointer in the `blaj` configuration file can be defined by listing these
offsets space delimited.

The following gif illustrates how to convert a Cheat Engine pointer into
a pointer in the configuration file:

![cheat_engine_pointer.gif](.doc-resources/cheat_engine_pointer.gif)

### `saveState` and `restoreState`

- Type: character
- Required: Yes

Set the keybind to save and to restore memory. (e.g. `saveState = 5` &
`restoreState = 6`) Sets the save state keybind to the keyboard key `5` and the
restore state keybind to the keyboard key `6`.

## `[Writer]`

The [Writer] section defines hex-encoded data to write to the target process
(for example, to set player position to a specific location).
This section is optional and can have multiple entries per configuration file.

### `<nickname>Pointer`

- Type: hexadecimal space delimited
- Required: Yes

The location in memory to write to. Can be prefixed with any name but must ends
with `Pointer` (e.g. `PlayerLocationPointer = 0x01C47590 0x70 0xF8`)

This is a similar structure to the `Pointer_#` parameter in the `[SaveRestore]`
section without the `_#`.

### `<nickname>Data`

- Type: hexadecimal bytes
- Required: Yes

The hexadecimal bytes to write at the memory location defined by the Pointer.
The parameter name must end with `Data` and be prefixed with the same prefix
used by the Pointer (e.g. `xPositionPointer` and `xPositionData`).

### `keybind`

- Type: character
- Required: Yes

Set the keybind to write the payload to the memory location of the Pointer.
Can be assigned to a single keyboard key (e.g. `keybind = p`) sets write keybind
to the keyboard key `p`.

## Troubleshooting

Logs are saved in the `.blaj` directory found in your home directory.
Any errors encountered will appear in the systray menu `Error Logs` and
will make the icon red.

## Thank you

Thankles to [Stephan Fox](https://github.com/stephen-fox) for helping me
so much with this project and being the best supporter I could ever ask for.
Thank you also to heki for brainstorming with me, testing, and providing a bunch
of the example configurations.
