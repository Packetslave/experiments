// Creates a new top-level project or tag. argv: [kind, name]
//   project <name>   new active project at the top level of the library
//   tag     <name>   new top-level tag
// Returns JSON {id, name}.
//
// Structural mutations go through Omni Automation (evaluateJavascript) —
// the AppleScript bridge is unreliable for them (see task_action.js move).
function run(argv) {
  const kind = argv[0];
  const name = argv[1];
  if (kind !== 'project' && kind !== 'tag') throw new Error('unknown kind: ' + kind);
  if (!name) throw new Error('name required');
  const app = Application('OmniFocus');
  const ctor = kind === 'project' ? 'Project' : 'Tag';
  return app.evaluateJavascript(
    '(() => {' +
    '  const o = new ' + ctor + '(' + JSON.stringify(name) + ');' +
    '  return JSON.stringify({id: o.id.primaryKey, name: o.name});' +
    '})()'
  );
}
