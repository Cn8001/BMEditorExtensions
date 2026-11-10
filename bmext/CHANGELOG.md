# Changelog

Each version of bmext, newest first: the extension format, the package
manager, the SDKs and bmx. The version is in VERSION; its section here
becomes the notes of its release (scripts/release-bmext.sh), and a version
is released only when it has one.

## 0.1.0

- The extension format (`bmext.json`), the package manager that installs
  extensions from a folder, a zip or a git repository, the backend SDK
  (`host`) and the frontend kit (`frontend/`).
- **bmx**, the tool for writing an extension: `bmx init` makes one that
  builds on its own, with a copy of the SDK; `bmx dev` builds it for your
  system and installs it in BMEditor; `bmx check` installs it from a clean
  clone of its files, as BMEditor does from git; `bmx pack` makes the zip
  to publish.
