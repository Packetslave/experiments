// Returns all tags as a JSON array.
// Uses columnar property access — see inbox.js for why.
function run() {
  const app = Application('OmniFocus');
  const doc = app.defaultDocument;
  const ts = doc.flattenedTags;
  const ids = ts.id();
  const names = ts.name();
  const out = ids.map(function (id, i) {
    return { id: id, name: names[i] };
  });
  return JSON.stringify(out);
}
