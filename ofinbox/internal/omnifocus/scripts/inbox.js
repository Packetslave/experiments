// Returns the OmniFocus inbox as a JSON array.
//
// Properties are fetched columnar-style (one Apple Event per property for
// the whole list, e.g. tasks.name() -> all names) instead of per-task.
// Per-task access costs one Apple Event per property per task, which takes
// ~45s on a ~200-item inbox; this form takes well under a second.
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
  const out = ids.map(function (id, i) {
    return {
      id: id,
      name: names[i],
      note: notes[i] || '',
      flagged: !!flagged[i],
      deferDate: defers[i] ? defers[i].toISOString() : null,
      dueDate: dues[i] ? dues[i].toISOString() : null,
      tags: tagNames[i] || [],
    };
  });
  return JSON.stringify(out);
}
