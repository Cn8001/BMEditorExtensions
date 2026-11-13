# C#

C# in BMEditor, made for Unity projects and UPM packages.

## What it gives

- **C# language server**: completion, errors, go to definition, usages,
  rename and quick fixes, through `csharp-ls` (or Roslyn's or OmniSharp's,
  whichever is installed) on the folder's solution.
- **Solution Explorer** (Solution panel): the solution as Visual Studio shows
  it. You get solution folders, projects, their references and files, and a
  filter. Click a file to open it. Next to a folder, New Script makes a
  MonoBehaviour, ScriptableObject, Custom Editor, Editor Window, class,
  interface, struct, enum or NUnit test, in the namespace of its assembly.
  A right click renames, deletes or makes a folder, an asset's .meta going
  along. Track Active Item selects the open file. In a Unity project only
  the project's own projects show, as in Visual Studio; the package button
  shows those of packages too, and the Explorer leaves out the .csproj, .sln
  and .meta files Unity writes.
- **Assembly graph** (Solution panel, Assemblies): every `.asmdef` and what it
  references, in layers. Loops are red. A reference to an assembly that is
  nowhere, such as a stale GUID, is marked. Click an assembly to see what it
  needs and what needs it; double-click to open it.
- **Inspector** (Inspector panel): the open `.asmdef`, `.asmref` or UPM
  `package.json` as a form. You can edit references (by name or by GUID),
  platforms, define constraints and version defines, or the package's
  name, version, Unity version, author, dependencies and samples. Each
  change is an ordinary edit of the file, so undo and save work as usual.
- **Unity compile errors in Problems** (Unity panel, Console): it follows
  Unity's `Editor.log` while Unity runs on the open project. Click an error
  to go to it.
- **.meta tools** (Unity panel, Meta) find:
  - files without a `.meta`;
  - `.meta` files whose asset is gone;
  - GUIDs used twice;
  - assets renamed or moved outside Unity.

  For a renamed or moved asset, the `.meta` is moved with it, which keeps
  the GUID and every reference to the asset. Unity: Find Asset by GUID and
  Unity: Copy GUID of This File work from the command palette.
- **Unity messages**: in a MonoBehaviour, ScriptableObject, Editor,
  EditorWindow or StateMachineBehaviour, typing `OnTri` offers
  `OnTriggerEnter(Collider other)` and the rest, with their signatures.
- **C# projects and solutions**: a folder with a `.sln`, `.slnx` or
  `.csproj` opens as a C# project, and New Project › C# makes a console
  app. The Solution Explorer opens the solution Project Settings › C# ›
  Solution names, else the one named like the folder, and lists every
  project in it, those outside the folder too, as Visual Studio does. With
  a `.sln` or `.csproj` open in a tab, C#: Show This Solution shows it
  there; the language server opens the same solution.
- **New Project › Unity Package**: a package with `package.json`, Runtime
  and Editor assemblies, tests and a sample. Run Unity: Check .meta Files
  on it once to give it its `.meta` files.
- Highlighting for `.asmdef`, `.csproj`, `.uxml`, `.uss` and Unity's YAML
  (`.meta`, `.prefab`, `.unity`, `.asset`...).

## Setting up

1. Install the .NET 10 SDK, then the language server:
   `dotnet tool install -g csharp-ls` (or `dotnet tool update -g csharp-ls`).
   Restart BMEditor so it is on PATH. csharp-ls loads projects with the
   SDK's MSBuild and must be the one built for that SDK: csharp-ls 0.16 on
   the .NET 8.0.4xx SDK loads no solution at all, and C# then completes
   only words. Output, under the language server, says when this happens.
   To use another one, set `BMEDITOR_CSHARP_SERVER` to `roslyn`
   (`roslyn-language-server`) or `omnisharp`.
2. A UPM package on its own, or a Unity project without a solution, needs
   nothing more: the extension writes the projects Unity would, one per
   `.asmdef`, against the Unity Editor that Unity Hub installed for the
   version in `package.json` or `ProjectVersion.txt` (or the one
   `BMEDITOR_UNITY_EDITOR` names), into its own folder, never yours.
   The Test Framework, NUnit and the packages in `dependencies` come from
   those the editor ships, so tests and their `Assert` complete too.
3. For a Unity project, Unity can also write the solution: in Unity, Edit ›
   Preferences › External Tools, choose Visual Studio or Visual Studio Code
   as the External Script Editor, tick the project files to generate, and
   press Regenerate project files. Open the project's folder in BMEditor.

Settings (Ctrl+,, C#): the path of Unity's `Editor.log` if it is not the
usual one, and whether new scripts get a `.meta`.

## Building

```
npm install
bmx dev      # build for this system and install it in BMEditor
bmx check    # install it from a clean clone, as BMEditor does from git
bmx pack     # csharp-<version>.zip for every system
```

From WSL, for BMEditor on Windows:
`bmx dev -target windows/amd64 -dir /mnt/c/Users/<you>/AppData/Roaming/bmeditor/extensions`.
