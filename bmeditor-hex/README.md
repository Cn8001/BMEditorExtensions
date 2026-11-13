# Hex Editor

A [BMEditor](https://github.com/Cn8001/BMEditor-Releases) extension that
opens the files that are not text, as [Bless](https://github.com/afrantzis/bless)
shows them. Images open in the editor itself.

- Offsets, sixteen bytes a row in two groups of eight, the same bytes as
  characters. Zero bytes are dim.
- Click a byte to select it; the bar below reads it as hex, decimal,
  binary and a character, and the bytes from it as 8, 16, 32 and 64-bit
  integers, in both byte orders, and as floats.
- Type over the selected byte: two hex digits in the hex column, or a
  character in the right column (Tab switches). Changed bytes show in
  colour; Ctrl+Z undoes, Ctrl+S saves. The file keeps its size: bytes are
  overwritten, never inserted or deleted.
- Arrows, Page Up and Down, Home and End move; Go to offset takes 0x1F0,
  1F0h or 496.
- Only the rows on screen are read, 64 KB at a time, so a file of any size
  opens at once. Reading and writing stay inside the open folder.

Needs BMEditor 0.1.5.

## Developing

```
npm install
bmx dev     # build for this system and install it in BMEditor, then reload the window
bmx check   # install it from a clean clone, as BMEditor does from git
bmx pack    # hex-<version>.zip, for every system, to publish
```
