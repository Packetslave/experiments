// Returns the OmniFocus inbox as a JSON array.
function run() {
  const app = Application('OmniFocus');
  const doc = app.defaultDocument;
  const tasks = doc.inboxTasks();
  const out = tasks.map(function (t) {
    let tags = [];
    try {
      tags = t.tags().map(function (x) { return x.name(); });
    } catch (e) {}
    let defer = null;
    let due = null;
    try { const d = t.deferDate(); if (d) defer = d.toISOString(); } catch (e) {}
    try { const d = t.dueDate(); if (d) due = d.toISOString(); } catch (e) {}
    return {
      id: t.id(),
      name: t.name(),
      note: t.note() || '',
      flagged: !!t.flagged(),
      deferDate: defer,
      dueDate: due,
      tags: tags,
    };
  });
  return JSON.stringify(out);
}
