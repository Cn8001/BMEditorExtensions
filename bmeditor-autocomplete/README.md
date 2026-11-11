# Autocomplete

Inline code suggestions in BMEditor, in every language: as you type, a
small code model on your machine suggests the rest of the line or the
next lines, drawn faint at the cursor. Nothing leaves the machine.

- **Tab** takes the suggestion, **Ctrl+Right** a word of it, **Esc** puts
  it away, **Alt+\\** asks for one now.
- Typing what the suggestion starts with keeps it.
- The top bar shows the model and what it is doing; a click pauses or
  resumes suggestions.

## Setting up

Run **Autocomplete: Get Model Ready** from the command palette (Ctrl+Shift+P),
or click the switch in the top bar. It downloads llama.cpp (about 25 MB)
and the model into your cache (`%LOCALAPPDATA%\bmeditor-autocomplete` on
Windows), then starts it. A broken download goes on where it stopped,
and each file is checked against its published SHA-256.

The extension itself is a few megabytes: one model fills in the middle of
any language, so nothing is per language, and only the model you choose
is downloaded.

| Model (Settings › Autocomplete › Model) | Size | |
|---|---|---|
| `0.5b` (default) | 0.5 GB | fast on any CPU |
| `1.5b` | 1.6 GB | better, wants a recent CPU |
| `3b` | 3.3 GB | best, slow without a GPU |

All are Qwen2.5-Coder, which fills in the middle. A model of your own
works too: the path or URL of a `.gguf` that fills in the middle.

With **Ollama** installed, set Model server to `ollama` and the Ollama
model to one you pulled (`ollama pull qwen2.5-coder:1.5b`): nothing is
downloaded.

The model runs only while you write: it stops after ten idle minutes and
when BMEditor closes. **Autocomplete: Remove Downloaded Models** gives the
disk space back.

Settings also take the CPU threads, how long typing pauses before a
suggestion is asked for, and languages to leave alone.

Needs BMEditor 0.1.5 or later.

## Building

```
npm install
bmx dev      # build for this system and install it in BMEditor
bmx check    # install it from a clean clone, as BMEditor does from git
bmx pack     # autocomplete-<version>.zip for every system
```
