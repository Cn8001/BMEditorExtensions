# Changelog

Each version of the C# extension, newest first. The section of a version
becomes the notes of its release, and a version is published only when it
has a section here.

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
