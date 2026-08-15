// Mutates a single task. argv: [action, taskID, arg?]
//   complete <id>            mark complete
//   drop     <id>            mark dropped
//   rename   <id> <name>     set name
//   flag     <id> true|false set flagged
//   defer    <id> <iso|"">   set defer date ("" clears)
//   due      <id> <iso|"">   set due date ("" clears)
//   move     <id> <projID>   move into project
//   tag      <id> <tagID>    add tag
function run(argv) {
  const action = argv[0];
  const taskID = argv[1];
  const app = Application('OmniFocus');
  const doc = app.defaultDocument;

  const matches = doc.flattenedTasks.whose({ id: taskID })();
  if (matches.length === 0) throw new Error('task not found: ' + taskID);
  const task = matches[0];

  switch (action) {
    case 'complete':
      app.markComplete(task);
      break;
    case 'drop':
      app.markDropped(task);
      break;
    case 'rename':
      task.name = argv[2];
      break;
    case 'flag':
      task.flagged = argv[2] === 'true';
      break;
    case 'defer':
      task.deferDate = argv[2] === '' ? null : new Date(argv[2]);
      break;
    case 'due':
      task.dueDate = argv[2] === '' ? null : new Date(argv[2]);
      break;
    case 'move': {
      const projs = doc.flattenedProjects.whose({ id: argv[2] })();
      if (projs.length === 0) throw new Error('project not found: ' + argv[2]);
      const proj = projs[0];
      try {
        app.move(task, { to: proj.tasks.end });
      } catch (e) {
        // Fallback for inbox items: assign a container and let cleanup file it.
        task.assignedContainer = proj;
        try { app.compact(doc); } catch (e2) {}
      }
      break;
    }
    case 'tag': {
      const tags = doc.flattenedTags.whose({ id: argv[2] })();
      if (tags.length === 0) throw new Error('tag not found: ' + argv[2]);
      app.add(tags[0], { to: task.tags });
      break;
    }
    default:
      throw new Error('unknown action: ' + action);
  }
  return 'ok';
}
