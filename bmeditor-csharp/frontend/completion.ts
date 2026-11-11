import { snippetCompletion, type Completion, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete'
import { EditorState } from '@codemirror/state'

// Completion of Unity's messages (Awake, Update, OnTriggerEnter, ...): the
// methods Unity calls by name, which a language server does not offer
// since nothing declares them. Offered where a member starts in a class
// that derives from something, with the method's whole signature.

interface UnityMessage {
  name: string
  params?: string
  returns?: string // void when unset
  override?: boolean // public override, for Editor and StateMachineBehaviour
  info: string
}

const lifecycle: UnityMessage[] = [
  { name: 'Awake', info: 'When the script instance is loaded, before Start.' },
  { name: 'OnEnable', info: 'When the object becomes enabled and active.' },
  { name: 'OnDisable', info: 'When the object becomes disabled or inactive.' },
  { name: 'OnDestroy', info: 'When the object is destroyed.' },
  { name: 'OnValidate', info: 'Editor only: when the script is loaded or a value changes in the Inspector.' },
  { name: 'Reset', info: 'Editor only: when the component is added or Reset is chosen.' },
]

const monoBehaviour: UnityMessage[] = [
  ...lifecycle,
  { name: 'Start', info: 'Before the first frame update, if the script is enabled.' },
  { name: 'Update', info: 'Every frame.' },
  { name: 'FixedUpdate', info: 'Every fixed frame-rate frame, for physics.' },
  { name: 'LateUpdate', info: 'Every frame, after all Update calls.' },
  { name: 'OnTriggerEnter', params: 'Collider other', info: 'When a Collider enters this trigger.' },
  { name: 'OnTriggerStay', params: 'Collider other', info: 'Once per physics update for each Collider touching this trigger.' },
  { name: 'OnTriggerExit', params: 'Collider other', info: 'When a Collider stops touching this trigger.' },
  { name: 'OnCollisionEnter', params: 'Collision collision', info: 'When this collider or rigidbody starts touching another.' },
  { name: 'OnCollisionStay', params: 'Collision collision', info: 'Once per frame while touching another collider or rigidbody.' },
  { name: 'OnCollisionExit', params: 'Collision collision', info: 'When this collider or rigidbody stops touching another.' },
  { name: 'OnTriggerEnter2D', params: 'Collider2D other', info: 'When a Collider2D enters this trigger (2D physics).' },
  { name: 'OnTriggerStay2D', params: 'Collider2D other', info: 'Each frame a Collider2D is inside this trigger (2D physics).' },
  { name: 'OnTriggerExit2D', params: 'Collider2D other', info: 'When a Collider2D leaves this trigger (2D physics).' },
  { name: 'OnCollisionEnter2D', params: 'Collision2D collision', info: 'When an incoming collider touches this one (2D physics).' },
  { name: 'OnCollisionStay2D', params: 'Collision2D collision', info: 'Each frame a collider touches this one (2D physics).' },
  { name: 'OnCollisionExit2D', params: 'Collision2D collision', info: 'When a collider stops touching this one (2D physics).' },
  { name: 'OnControllerColliderHit', params: 'ControllerColliderHit hit', info: 'When the CharacterController hits a collider while moving.' },
  { name: 'OnParticleCollision', params: 'GameObject other', info: 'When a particle hits a Collider.' },
  { name: 'OnJointBreak', params: 'float breakForce', info: 'When a joint attached to the same object breaks.' },
  { name: 'OnMouseDown', info: 'When the mouse button is pressed over this Collider.' },
  { name: 'OnMouseUp', info: 'When the mouse button is released.' },
  { name: 'OnMouseUpAsButton', info: 'When the mouse is released over the Collider it was pressed on.' },
  { name: 'OnMouseEnter', info: 'When the mouse enters this Collider.' },
  { name: 'OnMouseOver', info: 'Every frame while the mouse is over this Collider.' },
  { name: 'OnMouseExit', info: 'When the mouse is no longer over this Collider.' },
  { name: 'OnMouseDrag', info: 'While the mouse is held down after pressing on this Collider.' },
  { name: 'OnBecameVisible', info: 'When the renderer becomes visible to any camera.' },
  { name: 'OnBecameInvisible', info: 'When the renderer is no longer visible to any camera.' },
  { name: 'OnWillRenderObject', info: 'Once for each camera if the object is visible.' },
  { name: 'OnRenderObject', info: 'After the camera has rendered the scene.' },
  { name: 'OnDrawGizmos', info: 'To draw gizmos that are always drawn.' },
  { name: 'OnDrawGizmosSelected', info: 'To draw gizmos when the object is selected.' },
  { name: 'OnGUI', info: 'To render and handle immediate-mode GUI events.' },
  { name: 'OnApplicationFocus', params: 'bool hasFocus', info: 'When the application gains or loses focus.' },
  { name: 'OnApplicationPause', params: 'bool pauseStatus', info: 'When the application pauses or resumes.' },
  { name: 'OnApplicationQuit', info: 'Before the application quits.' },
  { name: 'OnAnimatorMove', info: 'To modify root motion each frame.' },
  { name: 'OnAnimatorIK', params: 'int layerIndex', info: 'To set up animation IK (inverse kinematics).' },
  { name: 'OnTransformParentChanged', info: 'When the parent of the transform changes.' },
  { name: 'OnTransformChildrenChanged', info: 'When the list of children of the transform changes.' },
  { name: 'OnRectTransformDimensionsChange', info: 'When the RectTransform changes size.' },
  { name: 'OnAudioFilterRead', params: 'float[] data, int channels', info: 'To intercept and change the audio data.' },
  { name: 'OnParticleSystemStopped', info: 'When all particles of the system have died.' },
]

const editor: UnityMessage[] = [
  { name: 'OnEnable', info: 'When the Editor is created or the selection changes to its target.' },
  { name: 'OnDisable', info: 'When the Editor goes out of scope.' },
  { name: 'OnDestroy', info: 'When the Editor is destroyed.' },
  { name: 'OnInspectorGUI', override: true, info: 'Draws the custom Inspector (IMGUI).' },
  { name: 'CreateInspectorGUI', override: true, returns: 'VisualElement', info: 'Builds the custom Inspector with UI Toolkit.' },
  { name: 'OnSceneGUI', info: 'Draws and handles events in the Scene view for the target.' },
  { name: 'HasPreviewGUI', override: true, returns: 'bool', info: 'Whether the Inspector shows a preview.' },
  { name: 'OnPreviewGUI', override: true, params: 'Rect r, GUIStyle background', info: 'Draws the preview.' },
]

const editorWindow: UnityMessage[] = [
  { name: 'OnEnable', info: 'When the window is loaded.' },
  { name: 'OnDisable', info: 'When the window goes out of scope.' },
  { name: 'OnDestroy', info: 'When the window is closed.' },
  { name: 'CreateGUI', info: 'Builds the window with UI Toolkit.' },
  { name: 'OnGUI', info: 'Draws the window (IMGUI).' },
  { name: 'Update', info: 'Many times a second on visible windows.' },
  { name: 'OnFocus', info: 'When the window gets keyboard focus.' },
  { name: 'OnLostFocus', info: 'When the window loses keyboard focus.' },
  { name: 'OnSelectionChange', info: 'When the selection changes.' },
  { name: 'OnHierarchyChange', info: 'When an object or group in the hierarchy changes.' },
  { name: 'OnProjectChange', info: 'When the project changes.' },
  { name: 'OnInspectorUpdate', info: '10 times a second, to update the window.' },
]

const animatorState = 'Animator animator, AnimatorStateInfo stateInfo, int layerIndex'
const stateMachineBehaviour: UnityMessage[] = [
  { name: 'OnStateEnter', override: true, params: animatorState, info: 'On the first frame a state is evaluated.' },
  { name: 'OnStateUpdate', override: true, params: animatorState, info: 'On each frame of a state, after the first and before the last.' },
  { name: 'OnStateExit', override: true, params: animatorState, info: 'On the last frame of a state.' },
  { name: 'OnStateMove', override: true, params: animatorState, info: 'Right after MonoBehaviour.OnAnimatorMove.' },
  { name: 'OnStateIK', override: true, params: animatorState, info: 'Right after MonoBehaviour.OnAnimatorIK.' },
  { name: 'OnStateMachineEnter', override: true, params: 'Animator animator, int stateMachinePathHash', info: 'When entering a state machine.' },
  { name: 'OnStateMachineExit', override: true, params: 'Animator animator, int stateMachinePathHash', info: 'When exiting a state machine.' },
]

// The messages for a class's base types; MonoBehaviour's for a base this
// cannot tell (a class of the project that derives from it, likely).
function messagesFor(bases: string[]): UnityMessage[] {
  const names = bases.map((b) => b.replace(/<.*$/, '').split('.').pop()!.trim())
  if (names.includes('ScriptableObject')) return lifecycle
  if (names.includes('Editor')) return editor
  if (names.includes('EditorWindow')) return editorWindow
  if (names.includes('StateMachineBehaviour')) return stateMachineBehaviour
  // Interfaces only (IDisposable, IComparable<T>...): not a Unity object.
  if (names.every((n) => /^I[A-Z]/.test(n))) return []
  return monoBehaviour
}

interface Scope {
  cls: boolean
  name: string // the class's, for ctor
  bases: string[]
}

const classHeader = /\b(?:class|struct)\s+(\w+)\s*(?:<[^>]*>)?\s*(?::\s*([^{]*?))?\s*(?:\bwhere\b[^{]*)?$/

// scopeAt follows braces from the start of the text to pos, leaving out
// comments and strings, and returns the scopes open at pos, innermost last.
function scopeAt(text: string): Scope[] {
  const stack: Scope[] = []
  let header = ''
  for (let i = 0; i < text.length; i++) {
    const c = text[i]
    const next = text[i + 1]
    if (c === '/' && next === '/') {
      const end = text.indexOf('\n', i)
      i = end < 0 ? text.length : end
      continue
    }
    if (c === '/' && next === '*') {
      const end = text.indexOf('*/', i + 2)
      i = end < 0 ? text.length : end + 1
      continue
    }
    if (c === '"' || c === "'") {
      const verbatim = text[i - 1] === '@'
      let j = i + 1
      for (; j < text.length; j++) {
        if (text[j] === c && (verbatim ? text[j + 1] !== c : text[j - 1] !== '\\')) break
        if (verbatim && text[j] === c) j++
        if (!verbatim && text[j] === '\n') break
      }
      i = j
      continue
    }
    if (c === '{') {
      const m = classHeader.exec(header.replace(/\[[^\]]*\]/g, ' '))
      stack.push({ cls: !!m, name: m?.[1] ?? '', bases: m?.[2] ? m[2].split(',').map((b) => b.trim()).filter(Boolean) : [] })
      header = ''
    } else if (c === '}') {
      stack.pop()
      header = ''
    } else if (c === ';') {
      header = ''
    } else {
      header += c
    }
  }
  return stack
}

// A member starts here: only modifiers, and maybe a return type, before
// the word being typed.
const memberStart = /^\s*(?:(?:public|private|protected|internal|static|override|virtual|async|new)\s+)*(void\s+|IEnumerator\s+)?$/

function declaration(m: UnityMessage, typedType: boolean): string {
  const params = m.params ?? ''
  if (m.override) return `${typedType ? '' : `public override ${m.returns ?? 'void'} `}${m.name}(${params})`
  return `${typedType ? '' : `${m.returns ?? 'void'} `}${m.name}(${params})`
}

function source(ctx: CompletionContext): CompletionResult | null {
  const word = ctx.matchBefore(/\w*/)
  if (!word || (word.from === word.to && !ctx.explicit)) return null
  const line = ctx.state.doc.lineAt(ctx.pos)
  const before = line.text.slice(0, word.from - line.from)
  const m = memberStart.exec(before)
  if (!m) return null
  const scopes = scopeAt(ctx.state.doc.sliceString(0, word.from))
  const inner = scopes[scopes.length - 1]
  if (!inner?.cls || !inner.bases.length) return null
  const text = ctx.state.doc.toString()
  const typedType = !!m[1]
  const options: Completion[] = messagesFor(inner.bases)
    .filter((msg) => !new RegExp(`\\b${msg.name}\\s*\\(`).test(text))
    .map((msg) => {
      const returns = msg.returns ?? 'void'
      const body = returns === 'void' ? '\t${}' : returns === 'bool' ? '\treturn ${false};' : '\t${}\n\treturn null;'
      return snippetCompletion(`${declaration(msg, typedType)}\n{\n${body}\n}`, {
        label: msg.name,
        detail: `(${msg.params ?? ''})`,
        info: msg.info,
        type: 'method',
        boost: -1,
      })
    })
  return options.length ? { from: word.from, options, validFor: /^\w*$/ } : null
}

// Snippets as Visual Studio has them: type prop, ctor, foreach... and take
// it with Tab or Enter; Tab then moves through the fields to fill in.
// Offered where a statement or a member starts. ${Name} fields with the
// same name are typed once for all.
interface CsSnippet {
  label: string
  detail: string
  body: string
  member?: boolean // only among a class's members
  statement?: boolean // only inside a method
}

const csSnippets: CsSnippet[] = [
  { label: 'prop', detail: 'property', member: true, body: 'public ${int} ${MyProperty} { get; set; }' },
  { label: 'propg', detail: 'property with a private setter', member: true, body: 'public ${int} ${MyProperty} { get; private set; }' },
  { label: 'propfull', detail: 'property with a field', member: true, body: 'private ${int} ${myField};\n\npublic ${int} ${MyProperty}\n{\n\tget => ${myField};\n\tset => ${myField} = value;\n}' },
  { label: 'ctor', detail: 'constructor', member: true, body: 'public ${Class}()\n{\n\t${}\n}' },
  { label: 'method', detail: 'method', member: true, body: 'private ${void} ${Name}(${})\n{\n\t${}\n}' },
  { label: 'for', detail: 'for loop', statement: true, body: 'for (int ${i} = 0; ${i} < ${length}; ${i}++)\n{\n\t${}\n}' },
  { label: 'forr', detail: 'reverse for loop', statement: true, body: 'for (int ${i} = ${length} - 1; ${i} >= 0; ${i}--)\n{\n\t${}\n}' },
  { label: 'foreach', detail: 'foreach loop', statement: true, body: 'foreach (var ${item} in ${collection})\n{\n\t${}\n}' },
  { label: 'if', detail: 'if statement', statement: true, body: 'if (${condition})\n{\n\t${}\n}' },
  { label: 'ifelse', detail: 'if else', statement: true, body: 'if (${condition})\n{\n\t${}\n}\nelse\n{\n\t${}\n}' },
  { label: 'while', detail: 'while loop', statement: true, body: 'while (${condition})\n{\n\t${}\n}' },
  { label: 'do', detail: 'do while loop', statement: true, body: 'do\n{\n\t${}\n} while (${condition});' },
  { label: 'switch', detail: 'switch statement', statement: true, body: 'switch (${value})\n{\n\tcase ${0}:\n\t\t${}\n\t\tbreak;\n\tdefault:\n\t\tbreak;\n}' },
  { label: 'try', detail: 'try catch', statement: true, body: 'try\n{\n\t${}\n}\ncatch (${Exception} ${e})\n{\n\t${}\n}' },
  { label: 'tryf', detail: 'try finally', statement: true, body: 'try\n{\n\t${}\n}\nfinally\n{\n\t${}\n}' },
  { label: 'using', detail: 'using statement', statement: true, body: 'using (var ${resource} = ${value})\n{\n\t${}\n}' },
  { label: 'lock', detail: 'lock statement', statement: true, body: 'lock (${this})\n{\n\t${}\n}' },
  { label: 'cw', detail: 'Console.WriteLine', statement: true, body: 'Console.WriteLine(${});' },
  { label: 'class', detail: 'class', body: 'public class ${Name}\n{\n\t${}\n}' },
  { label: 'struct', detail: 'struct', body: 'public struct ${Name}\n{\n\t${}\n}' },
  { label: 'interface', detail: 'interface', body: 'public interface ${IName}\n{\n\t${}\n}' },
  { label: 'enum', detail: 'enum', body: 'public enum ${Name}\n{\n\t${}\n}' },
  // Unity
  { label: 'dlog', detail: 'Debug.Log', statement: true, body: 'Debug.Log(${});' },
  { label: 'dlogw', detail: 'Debug.LogWarning', statement: true, body: 'Debug.LogWarning(${});' },
  { label: 'dloge', detail: 'Debug.LogError', statement: true, body: 'Debug.LogError(${});' },
  { label: 'getc', detail: 'GetComponent', statement: true, body: 'var ${name} = GetComponent<${Component}>();' },
  { label: 'sfield', detail: '[SerializeField] private field', member: true, body: '[SerializeField] private ${float} ${value};' },
  { label: 'header', detail: '[Header] attribute', member: true, body: '[Header("${Section}")]' },
  { label: 'tooltip', detail: '[Tooltip] attribute', member: true, body: '[Tooltip("${text}")]' },
  { label: 'range', detail: '[Range] attribute', member: true, body: '[Range(${0f}, ${1f})]' },
  { label: 'reqc', detail: '[RequireComponent]', body: '[RequireComponent(typeof(${Rigidbody}))]' },
  { label: 'menuitem', detail: '[MenuItem] static method', member: true, body: '[MenuItem("${Tools}/${Name}")]\nprivate static void ${Name}()\n{\n\t${}\n}' },
  { label: 'coroutine', detail: 'IEnumerator coroutine', member: true, body: 'private IEnumerator ${Routine}()\n{\n\t${}\n\tyield return ${null};\n}' },
]

function snippetSource(ctx: CompletionContext): CompletionResult | null {
  const word = ctx.matchBefore(/\w+/)
  if (!word && !ctx.explicit) return null
  const from = word?.from ?? ctx.pos
  const line = ctx.state.doc.lineAt(ctx.pos)
  if (line.text.slice(0, from - line.from).trim() !== '') return null // not where a statement or member starts
  const scopes = scopeAt(ctx.state.doc.sliceString(0, from))
  const inner = scopes[scopes.length - 1]
  const inClass = !!inner?.cls
  const inMethod = !!inner && !inner.cls && scopes.some((s) => s.cls)
  const options: Completion[] = csSnippets
    .filter((sn) => (sn.member ? inClass : sn.statement ? inMethod : !inMethod))
    .map((sn) => {
      const body = sn.label === 'ctor' && inner?.name ? sn.body.replace('${Class}', inner.name) : sn.body
      return snippetCompletion(body, {
        label: sn.label,
        detail: sn.detail,
        type: 'snippet',
        info: () => {
          const pre = document.createElement('pre')
          pre.className = 'cs-snippet-preview'
          pre.textContent = body.replace(/\$\{([^}]*)\}/g, '$1').replace(/\t/g, '    ')
          return pre
        },
        boost: -2,
      })
    })
  return options.length ? { from, options, validFor: /^\w*$/ } : null
}

export const unityCompletion = EditorState.languageData.of(() => [{ autocomplete: source }, { autocomplete: snippetSource }])
