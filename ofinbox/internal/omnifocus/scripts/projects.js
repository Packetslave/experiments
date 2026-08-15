// Returns filing destinations (active and on-hold projects) as a JSON array.
function run() {
  const app = Application('OmniFocus');
  const doc = app.defaultDocument;
  const projects = doc.flattenedProjects();
  const out = [];
  projects.forEach(function (p) {
    let status = '';
    try { status = String(p.status()); } catch (e) {}
    // Status strings come back like "active status" / "done status" depending
    // on the bridge; filter out anything finished either way.
    if (status.indexOf('done') !== -1 || status.indexOf('dropped') !== -1) return;
    let folder = '';
    try {
      const f = p.folder();
      if (f) folder = f.name();
    } catch (e) {}
    out.push({
      id: p.id(),
      name: p.name(),
      folder: folder,
      status: status.replace(' status', ''),
    });
  });
  return JSON.stringify(out);
}
