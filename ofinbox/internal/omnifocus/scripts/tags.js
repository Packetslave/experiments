// Returns all tags as a JSON array.
function run() {
  const app = Application('OmniFocus');
  const doc = app.defaultDocument;
  const tags = doc.flattenedTags();
  const out = tags.map(function (t) {
    return { id: t.id(), name: t.name() };
  });
  return JSON.stringify(out);
}
