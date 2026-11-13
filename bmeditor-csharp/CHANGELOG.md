# Changelog

Each version of the C# extension, newest first. The section of a version
becomes the notes of its release, and a version is published only when it
has a section here.

## 0.1.5

- In a C# project (Unity, a UPM package, a folder with a solution) the
  Explorer has a Files | Solution | Assemblies switch, as Rider has. Files
  is the folder, without what Unity writes for itself: Library, Temp,
  Logs, obj, UserSettings at the root, and the .csproj, .sln and .meta
  files. Solution is the Solution Explorer, Assemblies the assembly graph,
  each with the Inspector under it as Properties when an assembly
  definition or a package's package.json is open. The bottom panel keeps
  Unity. An editor before 0.1.5 keeps the panels as they were.
- The Solution Explorer opens on the solution with the open file
  selected, has one row of tools, joins a chain of folders that each hold
  only the next into one row (Assets/Scripts/Gameplay), shows a project's
  framework on hover only, and groups references into Projects, Packages
  and Assemblies, each closed, so a hundred of them stay out of the way.
- A .sln is highlighted: its projects, sections, GUIDs and settings.
- The assembly graph stays readable with many assemblies: a layer wider
  than the panel wraps onto more rows, and Fit zooms out no further than
  readable, starting at the graph's top.

## 0.1.4

- The Solution Explorer shows the project's own projects, as Visual Studio
  does for Unity: those of registry, git and built-in packages are hidden,
  and the package button shows them in a Packages folder.
- A right click on a row: New Script, New Folder, Rename, Delete, Copy Path,
  Open Project File. Renaming or deleting an asset takes its .meta along,
  so its GUID stays.
- Track Active Item selects the open file in the tree; Collapse All.
- In a Unity project the Explorer leaves out the .csproj, .sln and .meta
  files Unity writes. Settings › C# › Show Unity's generated files, or the
  solution's right-click menu, brings them back. Needs BMEditor 0.1.5.

## 0.1.3

- Unity packages, generated projects and Roslyn's logs go in BMEditor's
  data folder (`extension-data/csharp`) rather than the user's cache and
  temp folders; logs older than three days are removed.

## 0.1.2

- C# is coloured as Visual Studio colours it: namespaces, types, methods
  and members, attributes and constants, besides keywords and strings.
  Raw (`"""`), verbatim and interpolated strings and `#region` lines too.

## 0.1.1

- Named bmeditor-csharp in Extensions.

## 0.1.0

- C# with a language server: `csharp-ls`, Roslyn's or OmniSharp's, on the
  folder's solution.
- Solution Explorer: solution folders, projects, references and files,
  with New Script for MonoBehaviours, ScriptableObjects, editors, tests and more.
- Assembly graph of the `.asmdef` files, with loops and missing references marked.
- Inspector for `.asmdef`, `.asmref` and a UPM package's `package.json`.
- Unity's compile errors in Problems, from its `Editor.log`.
- .meta tools: missing, left over and doubled `.meta` files, and assets
  renamed outside Unity, with fixes. Find an asset by GUID, copy a file's GUID.
- Completion of Unity's messages (`Awake`, `OnTriggerEnter`, ...), and
  Visual Studio's snippets (`prop`, `ctor`, `foreach`...) with Unity's own
  (`dlog`, `sfield`, `coroutine`...), taken with Tab.
- Without a solution, a UPM package or a Unity project gets the projects
  Unity would write, one per `.asmdef`, for the language server and the
  Solution Explorer. What they reference and the folder has not, the Test
  Framework and NUnit above all, comes from the packages Unity Hub's
  editor ships, under Packages in the Solution Explorer.
- What the language server says, a solution that did not load above all,
  is in Output.
- A folder with a `.sln`, `.slnx` or `.csproj` opens as a C# project; New
  Project › C# makes a console app. Project Settings › C# › Solution picks
  the solution to open; C#: Show This Solution shows the one in the tab.
- New Project › Unity Package.
