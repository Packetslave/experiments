// Returns the direct children of one task as a JSON array. argv: [taskID]
//
// Same columnar-fetch approach as inbox.js: one Apple Event per property for
// the whole child list, never per child. Completed and dropped children are
// filtered out, same as inbox.js.
function run(argv) {
  const taskID = argv[0];
  const app = Application('OmniFocus');
  const doc = app.defaultDocument;
  const matches = doc.flattenedTasks.whose({ id: taskID })();
  if (matches.length === 0) throw new Error('task not found: ' + taskID);
  const tasks = matches[0].tasks;
  const ids = tasks.id();
  const names = tasks.name();
  const notes = tasks.note();
  const flagged = tasks.flagged();
  const defers = tasks.deferDate();
  const dues = tasks.dueDate();
  const tagNames = tasks.tags.name();
  const childCounts = tasks.numberOfTasks();
  const completedChildCounts = tasks.numberOfCompletedTasks();
  const completed = tasks.completed();
  const dropped = tasks.dropped();
  const out = [];
  for (let i = 0; i < ids.length; i++) {
    if (completed[i] || dropped[i]) continue;
    out.push({
      id: ids[i],
      name: names[i],
      note: notes[i] || '',
      flagged: !!flagged[i],
      deferDate: defers[i] ? defers[i].toISOString() : null,
      dueDate: dues[i] ? dues[i].toISOString() : null,
      tags: tagNames[i] || [],
      childCount: Math.max(0, (childCounts[i] || 0) - (completedChildCounts[i] || 0)),
      parentID: taskID,
    });
  }
  return JSON.stringify(out);
}
