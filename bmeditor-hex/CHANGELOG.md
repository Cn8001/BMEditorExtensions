# Changelog

Each version of the Hex Editor extension, newest first. The section of a
version becomes the notes of its release, and a version is published only
when it has a section here.

## 0.1.0

- Files that are not text open in hex, as Bless shows them, with what the
  selected bytes read as. Typing overwrites a byte, Ctrl+Z undoes, Ctrl+S
  saves; a file of any size opens at once. Needs BMEditor 0.1.5.
- A drag, Shift and Ctrl+A select bytes; a selection reads as UTF-8 and
  UTF-16 text too, for a string inside the file, and Ctrl+C copies it as
  hex or as characters.
