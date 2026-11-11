# Changelog

Each version of the Autocomplete extension, newest first. The section of a
version becomes the notes of its release, and a version is published only
when it has a section here.

## 0.1.2

- The model runs on the GPU: llama.cpp's Vulkan build (NVIDIA, AMD or
  Intel) with every layer on the card. Settings › Autocomplete › Run the
  model on the GPU turns it off.
- The top bar's Autocomplete button opens a menu: pick the model (it
  downloads and starts), the GPU, pause, turn off. Nothing is typed.
- In Settings, Model server and Model are dropdowns, and the Ollama
  settings show only while Ollama is the server. A `.gguf` of your own
  goes in Model file. Needs BMEditor 0.1.6.
- A cancelled suggestion no longer shows a warning in the top bar.
- A keystroke while llama.cpp loads no longer kills it, so it gets ready
  even while you type.

## 0.1.1

- llama.cpp downloads straight from its release, with its size and
  SHA-256 known in advance: no call to GitHub's API, which failed when
  api.github.com could not be reached and has a rate limit.
- A download cut by the network (a failed lookup, a dropped connection)
  is tried again three times, going on from where it stopped.

## 0.1.0

- Inline suggestions (ghost text) in every language from a code model on
  the machine: Tab takes one, Ctrl+Right a word, Esc puts it away, Alt+\
  asks.
- llama.cpp with Qwen2.5-Coder 0.5B, 1.5B or 3B, downloaded on request
  into the cache and checked, or a `.gguf` of one's own; or Ollama.
- The model starts when needed and stops when idle or with BMEditor.
