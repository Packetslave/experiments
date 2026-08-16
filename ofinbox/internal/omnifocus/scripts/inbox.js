// Returns the OmniFocus inbox as a JSON array.
//
// Properties are fetched columnar-style (one Apple Event per property for
// the whole list, e.g. tasks.name() -> all names) instead of per-task.
// Per-task access costs one Apple Event per property per task, which takes
// ~45s on a ~200-item inbox; this form takes well under a second.
//
// doc.inboxTasks includes completed and dropped items (the OmniFocus UI
// hides them behind its "remaining" view filter), so filter them out here.
function run() {
  const app = Application('OmniFocus');
  const doc = app.defaultDocument;
  const tasks = doc.inboxTasks;
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
    });
  }
  return JSON.stringify(out);
}
