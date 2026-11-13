# ASCDII Engine

A [BMEditor](https://github.com/Cn8001/BMEditor-Releases) extension for games
made with the [ASCDII Engine](https://github.com/Cn8001/ASCDII-Engine): a
folder with an `.ascdiiproj`.

- **Game view in the Explorer.** The project's assets by kind: scripts,
  scenes, prefabs, materials, models, textures, sprites, animators, audio,
  then installed packages. A script lists the scene and prefab objects that
  use it; a click opens the scene at that object. The start scene is marked,
  and `.meta` files are hidden from the Files view too.
- **Lua with the engine's API.** Completion and hover for the globals
  (`find`, `spawn`, `key_down`, `play_sound`, ...), object and component
  members after `:` and `.`, `camera`, `env`, `pipeline`, the drawer of
  `draw(self, d)`, `vec3`, the field constructors, and `self.` with the
  script's own fields. `function ` offers the hooks with their parameters.
  Syntax errors show in Problems as you type, from the engine's own Lua.
- **Play, as Unity has it.** The top bar's Play runs the game in its window,
  or opens the project in the ASCDII editor, as the menu beside it picks;
  Stop ends it, Build runs `ascdii build`. The game's output goes to
  Output, and a script that fails puts its error in Problems. The engine
  reloads a script you save while the game runs.

It runs the engine's `ascdii` program: get the engine from
[github.com/Cn8001/ASCDII-Engine](https://github.com/Cn8001/ASCDII-Engine),
then set the program in Settings, or have it on the PATH.

## Developing

```
npm install
bmx dev     # build for this system and install it in BMEditor, then reload the window
bmx check   # install it from a clean clone, as BMEditor does from git
bmx pack    # ascdii-<version>.zip, for every system, to publish
```
