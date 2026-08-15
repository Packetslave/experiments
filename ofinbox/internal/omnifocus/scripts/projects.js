// Returns filing destinations (active and on-hold projects) as a JSON array.
// Uses columnar property access — see inbox.js for why.
function run() {
  const app = Application('OmniFocus');
  const doc = app.defaultDocument;
  const ps = doc.flattenedProjects;
  const ids = ps.id();
  const names = ps.name();
  const statuses = ps.status();
  const folders = ps.folder.name(); // null for top-level projects
  const out = [];
  ids.forEach(function (id, i) {
    // Status strings come back like "active status" / "done status" depending
    // on the bridge; filter out anything finished either way.
    const status = String(statuses[i]);
    if (status.indexOf('done') !== -1 || status.indexOf('dropped') !== -1) return;
    out.push({
      id: id,
      name: names[i],
      folder: folders[i] || '',
      status: status.replace(' status', ''),
    });
  });
  return JSON.stringify(out);
}
