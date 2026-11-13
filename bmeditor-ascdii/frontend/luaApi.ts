// The ASCDII Engine's Lua API, as docs/usage/lua-api.html describes it, for
// completion and hover. sig is what a call looks like; info says what it
// does.

export interface Entry {
  name: string
  sig?: string
  info: string
  type: 'function' | 'method' | 'property' | 'variable' | 'keyword' | 'constant'
}

const fn = (name: string, sig: string, info: string): Entry => ({ name, sig, info, type: 'function' })
const method = (name: string, sig: string, info: string): Entry => ({ name, sig, info, type: 'method' })
const prop = (name: string, info: string): Entry => ({ name, info, type: 'property' })

export const globals: Entry[] = [
  fn('find', 'find(name)', 'The first object with that name, or nil.'),
  fn('find_all', 'find_all(name)', 'Every object with that name, as a list.'),
  fn('find_by_tag', 'find_by_tag(tag)', 'Every object with that tag.'),
  fn('find_by_component', 'find_by_component(kind)', 'Every object that has a component of that kind, such as "light".'),
  fn('find_by_prefab', 'find_by_prefab(prefab)', 'Every instance of a prefab, given as a path such as "prefabs/lantern.prefab.json" or a prefab() field.'),
  fn('create', 'create(name, parent)', 'A new empty object; parent is optional.'),
  fn('spawn', 'spawn(prefab, pos, parent)', 'An instance of a prefab at pos (a vec3, or x, y, z) in the world; with parent it becomes that object\'s child.'),
  fn('destroy', 'destroy(obj)', 'Removes an object and its children, after their on_destroy hooks.'),
  fn('key_down', 'key_down(k)', 'Whether a key is held: "w", "space", "up", "enter".'),
  fn('key_pressed', 'key_pressed(k)', 'Whether a key went down this frame.'),
  fn('time', 'time()', 'Scene time in seconds.'),
  fn('time_scale', 'time_scale(k)', 'Sets the game speed when given a number (0 pauses, 0.5 is slow motion) and returns it.'),
  fn('log', 'log(...)', 'Writes to the Console, tagged with the script\'s file.'),
  fn('print', 'print(...)', 'Same as log.'),
  fn('say', 'say(text, seconds)', 'A label over the script\'s own object; say(obj, text, seconds) over another.'),
  fn('tween', 'tween(obj, prop, to, seconds, ease)', 'Same as obj:tween. Eases: linear, in, out, inout, step, back, elastic.'),
  fn('play_sound', 'play_sound(sound, volume)', 'Plays a sound asset once, by path or from a field.'),
  fn('quit', 'quit()', 'Ends the game.'),
  fn('vec3', 'vec3(x, y, z)', 'A vector; missing components are 0. +, -, *, / with vectors or numbers.'),
  fn('dist', 'dist(a, b)', 'Earlier API: distance between two objects on the ground.'),
  fn('hour', 'hour()', 'Earlier API: the time of day.'),
  fn('setHour', 'setHour(h)', 'Earlier API: sets the time of day.'),
  { name: 'camera', info: 'The game camera: target, span, pitch, yaw, anchorX, anchorY, object, follow(obj, rate).', type: 'variable' },
  { name: 'env', info: 'The scene\'s environment: any environment property, such as env.hour or env.auto.', type: 'variable' },
  { name: 'pipeline', info: 'The render pipeline: pass(name), enable(name, on), enabled(name).', type: 'variable' },
  { name: 'mods', info: 'mods.emit(name, ...) sends an event the mod API declared to every mod.', type: 'variable' },
  { name: 'api', info: 'In the mod API script: func, value and event declare what mods reach.', type: 'variable' },
  { name: 'fields', info: 'The values the Inspector edits, read in source order: fields = { speed = 4, target = object() }.', type: 'variable' },
]

// Field constructors, for the fields table.
export const fieldTypes: Entry[] = [
  fn('range', 'range(v, min, max)', 'A slider: a number between min and max.'),
  fn('int', 'int(v)', 'A whole number.'),
  fn('enum', 'enum("a", "b")', 'A drop down: the chosen string.'),
  fn('color', 'color("#rrggbb")', 'A color picker: a hex string.'),
  fn('object', 'object()', 'An object slot: an object or nil.'),
  fn('component', 'component(kind)', 'A component slot: a component, a script\'s self, or nil.'),
  fn('prefab', 'prefab()', 'A prefab slot: a reference to pass to spawn.'),
]

// The hooks the engine calls on a script, as functions it defines.
export const hooks: Entry[] = [
  fn('start', 'start(self)', 'After the script loads, and after every hot reload.'),
  fn('update', 'update(self, dt)', 'Every frame, after the key hooks; dt is the frame\'s seconds.'),
  fn('key', 'key(self, k)', 'Once per key press, before update. Return true to keep the key from later components of the object.'),
  fn('on_collision', 'on_collision(self, other)', 'When a contact with another object begins.'),
  fn('on_trigger', 'on_trigger(self, other)', 'When a contact begins and either shape is a trigger.'),
  fn('on_destroy', 'on_destroy(self)', 'When Lua destroys the object or an ancestor, before it goes.'),
  fn('draw', 'draw(self, d)', 'Every frame, to draw more shapes with d, in the object\'s local space.'),
]

// What an object handle has: self, find(...), an object() field.
export const objectMembers: Entry[] = [
  prop('name', 'Name; read and write.'),
  prop('tag', 'Tag; read and write.'),
  prop('active', 'Visibility; hidden objects and their children are not drawn.'),
  prop('solid', 'Blocks the player with the object\'s bounds.'),
  prop('parent', 'Parent object or nil; setting it reparents and keeps the world position.'),
  prop('children', 'List of child objects.'),
  prop('root', 'The topmost parent, or the object itself.'),
  prop('prefab', 'The prefab\'s path for an instance, otherwise nil.'),
  prop('id', 'Runtime id, unique while the game runs.'),
  prop('position', 'Position in the world, a vec3.'),
  prop('local_position', 'Position relative to the parent.'),
  prop('rotation', 'Degrees around X, Y and Z in the world.'),
  prop('local_rotation', 'Degrees relative to the parent.'),
  prop('scale', 'Scale relative to the parent.'),
  prop('local_scale', 'Scale relative to the parent; same as scale.'),
  prop('world_scale', 'Scale after all parents.'),
  prop('forward', 'The object\'s X direction in the world.'),
  prop('right', 'The object\'s Y direction in the world.'),
  prop('up', 'The object\'s Z direction in the world.'),
  method('set_parent', 'o:set_parent(p, keep_world)', 'Makes the object a child of p, or top level when p is nil. keep_world defaults to true.'),
  method('translate', 'o:translate(v, space)', 'Moves along the object\'s own axes, or the world\'s with "world".'),
  method('rotate', 'o:rotate(degrees)', 'Adds a vec3 of degrees to the local rotation.'),
  method('look_at', 'o:look_at(target)', 'Turns around Z to face an object or a vec3 on the ground plane.'),
  method('transform_point', 'o:transform_point(p)', 'A point from the object\'s space to the world.'),
  method('inverse_transform_point', 'o:inverse_transform_point(p)', 'A point from the world to the object\'s space.'),
  method('move', 'o:move(delta)', 'Moves by a world offset: a vec3 or x, y, z.'),
  method('move_to', 'o:move_to(pos)', 'Moves to a world position: a vec3 or x, y, z.'),
  method('component', 'o:component(name)', 'A component by kind ("light"), by key ("light#2") or a script by file name ("door"). Scripts return their self.'),
  method('components', 'o:components(kind)', 'All components, or all of one kind.'),
  method('add_component', 'o:add_component(kind)', 'Adds and returns a new component.'),
  method('remove_component', 'o:remove_component(c)', 'Removes a component handle or script.'),
  method('child', 'o:child(path)', 'A descendant by name path, such as "cage/flame".'),
  method('get', 'o:get(prop)', 'Any property by name: "pos", "light.radius", or a short name such as "glow".'),
  method('set', 'o:set(prop, value)', 'Sets any property by name.'),
  method('distance', 'o:distance(other)', 'World distance between two objects.'),
  method('tween', 'o:tween(prop, to, seconds, ease)', 'Animates a property from its current value. Eases: linear, in, out, inout, step, back, elastic.'),
  method('say', 'o:say(text, seconds)', 'Shows a label over the object.'),
  method('play', 'o:play(clip)', 'Starts one of the object\'s animation clips by name. On a component: animator, audio source or emitter.'),
  method('stop', 'o:stop(clip)', 'Stops one of the object\'s animation clips by name.'),
  method('destroy', 'o:destroy()', 'Same as destroy(o).'),
]

// What component handles add: every one has enabled, kind and object, and
// these kinds have methods.
export const componentMembers: Entry[] = [
  prop('enabled', 'The component card\'s checkbox; read and write.'),
  prop('kind', 'The component kind.'),
  prop('object', 'The object it belongs to.'),
  method('trigger', 'a:trigger(name)', 'Animator: fires a trigger parameter.'),
  method('playing', 's:playing()', 'Audio source: whether it plays.'),
  method('add_force', 'rb:add_force(vec3)', 'Rigidbody: adds a force.'),
  method('emit', 'p:emit(n)', 'Emitter: emits n particles now.'),
  method('clear', 'p:clear()', 'Emitter: removes its particles.'),
  method('count', 'p:count()', 'Emitter: how many particles are alive.'),
  prop('velocity', 'Rigidbody: velocity, a vec3.'),
  prop('mass', 'Rigidbody: mass.'),
  prop('gravity', 'Rigidbody: whether gravity pulls it.'),
  prop('kinematic', 'Rigidbody: moved by scripts, not by forces.'),
  prop('volume', 'Audio source: volume.'),
  prop('loop', 'Audio source: whether it loops.'),
  prop('clip', 'Audio source: the sound it plays.'),
  prop('state', 'Animator: the active state.'),
]

export const vec3Members: Entry[] = [
  prop('x', 'The X component.'),
  prop('y', 'The Y component.'),
  prop('z', 'The Z component.'),
  method('len', 'v:len()', 'Length.'),
  method('norm', 'v:norm()', 'The vector scaled to length 1.'),
  method('dist', 'v:dist(b)', 'Distance to b.'),
  method('dot', 'v:dot(b)', 'Dot product.'),
  method('cross', 'v:cross(b)', 'Cross product.'),
  method('lerp', 'v:lerp(b, t)', 'The point t of the way to b.'),
]

// The d of draw(self, d); positions and sizes are in the object's space.
export const drawerMembers: Entry[] = [
  method('box', 'd:box(pos, size, color, glow)', 'A box standing on pos.'),
  method('blob', 'd:blob(pos, radius, color, glow)', 'A sphere resting on pos.'),
  method('cylinder', 'd:cylinder(pos, radius, height, color)', 'An upright cylinder.'),
  method('cone', 'd:cone(pos, radius, height, color)', 'An upright cone.'),
  method('point', 'd:point(pos, size, color, glow)', 'A glowing dot, for sparks and stars.'),
  method('light', 'd:light(pos, radius, color, intensity)', 'A point light.'),
  method('shadow', 'd:shadow(pos, rx, ry)', 'A soft contact shadow.'),
]

// The members of the global tables, after a dot.
export const tables: Record<string, Entry[]> = {
  camera: [
    prop('target', 'The point the camera looks at; on a camera object, its world position.'),
    prop('span', 'World units across the width of the view.'),
    prop('pitch', 'Angle above the ground, in degrees.'),
    prop('yaw', 'Rotation around the up axis, in degrees.'),
    prop('anchorX', 'Screen position of the target, 0 to 1.'),
    prop('anchorY', 'Screen position of the target, 0 to 1.'),
    prop('object', 'The camera\'s GameObject, or nil in a scene without a camera.'),
    fn('follow', 'camera.follow(obj, rate)', 'Moves the camera toward obj every frame; camera.follow(nil) stops.'),
  ],
  env: [prop('hour', 'The time of day, 0 to 24.'), prop('auto', 'Whether the time of day runs by itself.')],
  pipeline: [
    fn('pass', 'pipeline.pass(name)', 'A render pass, whose settings are fields: pipeline.pass("fog").density = 0.8.'),
    fn('enable', 'pipeline.enable(name, on)', 'Turns a pass on or off.'),
    fn('enabled', 'pipeline.enabled(name)', 'Whether a pass is on.'),
  ],
  mods: [fn('emit', 'mods.emit(name, ...)', 'Sends an event the mod API declared to every mod; nothing when mods are disabled.')],
  api: [
    fn('func', 'api.func(name, fn)', 'A function mods call as game.<name>(...).'),
    fn('value', 'api.value(name, fn)', 'A value mods read as game.<name>().'),
    fn('event', 'api.event(name)', 'An event mods handle as on_<name>(...).'),
  ],
  math: ['abs', 'acos', 'asin', 'atan', 'atan2', 'ceil', 'cos', 'cosh', 'deg', 'exp', 'floor', 'fmod', 'frexp', 'huge', 'ldexp', 'log', 'log10', 'max', 'min', 'modf', 'pi', 'pow', 'rad', 'random', 'randomseed', 'sin', 'sinh', 'sqrt', 'tan', 'tanh'].map((n) =>
    n === 'pi' || n === 'huge' ? { name: n, info: `math.${n}`, type: 'constant' as const } : fn(n, `math.${n}(...)`, 'Lua 5.1 math library.'),
  ),
  string: ['byte', 'char', 'find', 'format', 'gmatch', 'gsub', 'len', 'lower', 'match', 'rep', 'reverse', 'sub', 'upper'].map((n) => fn(n, `string.${n}(...)`, 'Lua 5.1 string library.')),
  table: ['concat', 'insert', 'maxn', 'remove', 'sort'].map((n) => fn(n, `table.${n}(...)`, 'Lua 5.1 table library.')),
}

// What Lua 5.1 itself offers a script: the engine opens base, table,
// string and math only.
export const builtins: Entry[] = [
  'assert', 'error', 'getmetatable', 'ipairs', 'next', 'pairs', 'pcall', 'rawequal', 'rawget', 'rawset', 'select', 'setmetatable', 'tonumber', 'tostring', 'type', 'unpack', 'xpcall',
].map((n) => fn(n, `${n}(...)`, 'Lua 5.1 base library.'))

export const keywords: Entry[] = ['and', 'break', 'do', 'else', 'elseif', 'end', 'false', 'for', 'function', 'if', 'in', 'local', 'nil', 'not', 'or', 'repeat', 'return', 'then', 'true', 'until', 'while'].map(
  (n) => ({ name: n, info: 'Lua keyword', type: 'keyword' as const }),
)
