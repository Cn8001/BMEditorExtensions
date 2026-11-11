import { snippetCompletion, type Completion, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete'
import { EditorState, type Extension } from '@codemirror/state'

// Snippets as VS Code's PowerShell extension has them, and cmd's own:
// type the name where a statement starts and take it with Tab or Enter;
// Tab then moves through the fields. ${name} fields with the same name are
// typed once for all.

export interface Snippet {
  label: string
  detail: string
  body: string
}

export const powershellSnippets: Snippet[] = [
  { label: 'function', detail: 'function', body: 'function ${Verb-Noun} {\n\tparam (\n\t\t${}\n\t)\n\t${}\n}' },
  {
    label: 'function-advanced',
    detail: 'advanced function, with help',
    body:
      'function ${Verb-Noun} {\n\t<#\n\t.SYNOPSIS\n\t\t${What it does}\n\t.EXAMPLE\n\t\t${Verb-Noun} -Name value\n\t#>\n\t[CmdletBinding()]\n\tparam (\n\t\t[Parameter(Mandatory, ValueFromPipeline)]\n\t\t[string]$${Name}\n\t)\n\n\tbegin {\n\t}\n\n\tprocess {\n\t\t${}\n\t}\n\n\tend {\n\t}\n}',
  },
  { label: 'param', detail: 'param block', body: 'param (\n\t[Parameter(Mandatory)]\n\t[${string}]$${Name}\n)' },
  { label: 'parameter', detail: 'a parameter', body: '[Parameter(${Mandatory})]\n[${string}]$${Name}' },
  { label: 'cmdletbinding', detail: '[CmdletBinding()] with param', body: '[CmdletBinding()]\nparam (\n\t${}\n)' },
  { label: 'if', detail: 'if', body: 'if (${condition}) {\n\t${}\n}' },
  { label: 'ifelse', detail: 'if else', body: 'if (${condition}) {\n\t${}\n}\nelse {\n\t${}\n}' },
  { label: 'elseif', detail: 'elseif', body: 'elseif (${condition}) {\n\t${}\n}' },
  { label: 'foreach', detail: 'foreach loop', body: 'foreach ($${item} in $${collection}) {\n\t${}\n}' },
  { label: 'foreach-object', detail: 'ForEach-Object in a pipeline', body: '| ForEach-Object {\n\t$_${}\n}' },
  { label: 'for', detail: 'for loop', body: 'for ($${i} = 0; $${i} -lt $${array}.Count; $${i}++) {\n\t${}\n}' },
  { label: 'while', detail: 'while loop', body: 'while (${condition}) {\n\t${}\n}' },
  { label: 'do-while', detail: 'do while loop', body: 'do {\n\t${}\n} while (${condition})' },
  { label: 'do-until', detail: 'do until loop', body: 'do {\n\t${}\n} until (${condition})' },
  { label: 'switch', detail: 'switch', body: "switch (${value}) {\n\t${'a'} { ${} }\n\tDefault { ${} }\n}" },
  { label: 'try', detail: 'try catch', body: 'try {\n\t${}\n}\ncatch {\n\tWrite-Error $_\n}' },
  { label: 'tryf', detail: 'try catch finally', body: 'try {\n\t${}\n}\ncatch {\n\tWrite-Error $_\n}\nfinally {\n\t${}\n}' },
  { label: 'class', detail: 'class', body: 'class ${Name} {\n\t[${string}]$${Property}\n\n\t${Name}() {\n\t\t${}\n\t}\n}' },
  { label: 'enum', detail: 'enum', body: 'enum ${Name} {\n\t${Value}\n}' },
  { label: 'pscustomobject', detail: '[pscustomobject]', body: '[pscustomobject]@{\n\t${Name} = ${value}\n}' },
  { label: 'hashtable', detail: 'hashtable', body: '$${table} = @{\n\t${Key} = ${value}\n}' },
  { label: 'splat', detail: 'splatting', body: '$${params} = @{\n\t${Path} = ${value}\n}\n${Get-Item} @${params}' },
  { label: 'region', detail: '#region', body: '#region ${Name}\n${}\n#endregion' },
  { label: 'comment-help', detail: 'comment-based help', body: '<#\n.SYNOPSIS\n\t${What it does}\n.DESCRIPTION\n\t${}\n.EXAMPLE\n\t${}\n#>' },
  { label: 'strict', detail: 'Set-StrictMode and $ErrorActionPreference', body: "Set-StrictMode -Version Latest\n$ErrorActionPreference = 'Stop'\n" },
  { label: 'admin', detail: 'stop unless run as administrator', body: '#Requires -RunAsAdministrator\n' },
  { label: 'describe', detail: 'Pester Describe with It', body: "Describe '${Name}' {\n\tIt '${does something}' {\n\t\t${} | Should -Be ${expected}\n\t}\n}" },
  { label: 'it', detail: 'Pester It', body: "It '${does something}' {\n\t${} | Should -Be ${expected}\n}" },
  { label: 'context', detail: 'Pester Context', body: "Context '${when}' {\n\t${}\n}" },
]

export const batchSnippets: Snippet[] = [
  { label: 'header', detail: "@echo off, setlocal, to the file's folder", body: '@echo off\nsetlocal enableextensions\npushd "%~dp0"\n\n${}\n\npopd\nendlocal\n' },
  { label: 'sub', detail: 'label called as a subroutine', body: ':${name}\n${}\ngoto :eof' },
  { label: 'callsub', detail: 'call a label with arguments', body: 'call :${name} ${args}' },
  { label: 'if', detail: 'if', body: 'if ${condition} (\n\t${}\n)' },
  { label: 'ifelse', detail: 'if else', body: 'if ${condition} (\n\t${}\n) else (\n\t${}\n)' },
  { label: 'ifexist', detail: 'if a file is there', body: 'if exist "${path}" (\n\t${}\n)' },
  { label: 'ifdefined', detail: 'set a variable unless it is set', body: 'if not defined ${name} set "${name}=${value}"' },
  { label: 'iferror', detail: 'stop when the last command failed', body: 'if errorlevel 1 (\n\techo ${Failed} 1>&2\n\texit /b %errorlevel%\n)' },
  { label: 'for', detail: 'for each file', body: 'for %%${f} in (${*.txt}) do (\n\techo %%~nx${f}\n)' },
  { label: 'forr', detail: 'for each file under a folder', body: 'for /r "${.}" %%${f} in (${*.txt}) do (\n\techo %%~f${f}\n)' },
  { label: 'ford', detail: 'for each folder', body: 'for /d %%${d} in (${*}) do (\n\techo %%~nx${d}\n)' },
  { label: 'forl', detail: 'for each number', body: 'for /l %%${1:i} in (${2:1},1,${3:10}) do (\n\techo %%${1:i}\n)' },
  { label: 'forf', detail: "for each line of a command's output", body: 'for /f "usebackq delims=" %%${l} in (`${command}`) do (\n\techo %%${l}\n)' },
  { label: 'setvar', detail: 'set a variable', body: 'set "${name}=${value}"' },
  { label: 'seta', detail: 'set /a arithmetic', body: 'set /a ${n}=${n}+1' },
  { label: 'input', detail: 'read a line typed', body: 'set /p ${answer}=${Question}: ' },
  { label: 'choice', detail: 'ask yes or no', body: 'choice /c yn /m "${Go on?}"\nif errorlevel 2 goto :eof' },
  { label: 'admin', detail: 'stop unless run as administrator', body: 'net session >nul 2>&1\nif errorlevel 1 (\n\techo Run this as an administrator. 1>&2\n\texit /b 1\n)' },
  { label: 'wait', detail: 'wait some seconds', body: 'timeout /t ${1:5} /nobreak >nul' },
  { label: 'quiet', detail: 'output to nowhere', body: '>nul 2>&1' },
  { label: 'ps', detail: 'run PowerShell', body: 'powershell -NoProfile -ExecutionPolicy Bypass -Command "${}"' },
]

function source(snippets: Snippet[], startsStatement: (before: string) => boolean) {
  return (ctx: CompletionContext): CompletionResult | null => {
    const word = ctx.matchBefore(/[\w-]+/)
    if (!word && !ctx.explicit) return null
    const from = word?.from ?? ctx.pos
    const line = ctx.state.doc.lineAt(ctx.pos)
    if (!startsStatement(line.text.slice(0, from - line.from))) return null
    const options: Completion[] = snippets.map((sn) =>
      snippetCompletion(sn.body, {
        label: sn.label,
        detail: sn.detail,
        type: 'snippet',
        info: () => {
          const pre = document.createElement('pre')
          pre.className = 'ps-snippet-preview'
          pre.textContent = sn.body.replace(/\$\{([^}]*)\}/g, '$1').replace(/\t/g, '    ')
          return pre
        },
        boost: -2,
      }),
    )
    return { from, options, validFor: /^[\w-]*$/ }
  }
}

// A PowerShell statement starts a line, or follows ; { or a pipe.
export const powershellCompletion: Extension = EditorState.languageData.of(() => [
  { autocomplete: source(powershellSnippets, (b) => /(^|[;{|])\s*$/.test(b)) },
])

// A batch statement starts a line, or follows ( & && ||.
export const batchCompletion: Extension = EditorState.languageData.of(() => [
  { autocomplete: source(batchSnippets, (b) => /(^\s*@?|[(&|]\s*)$/.test(b)) },
])
