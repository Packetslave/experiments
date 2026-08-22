// Returns past filing decisions as a JSON array: every task that lives in a
// project (remaining or completed — dropped excluded), with the project's ID
// and the task's tag IDs. Used to score project/tag suggestions.
// Uses columnar property access — see inbox.js for why.
function run() {
  const app = Application('OmniFocus');
  const doc = app.defaultDocument;
  const ts = doc.flattenedTasks;
  const names = ts.name();
  const projIDs = ts.containingProject.id(); // null for inbox items
  const dropped = ts.dropped();
  const tagIDs = ts.tags.id();
  const out = [];
  for (let i = 0; i < names.length; i++) {
    if (dropped[i] || !projIDs[i]) continue;
    out.push({ name: names[i], projectID: projIDs[i], tags: tagIDs[i] || [] });
  }
  return JSON.stringify(out);
}
