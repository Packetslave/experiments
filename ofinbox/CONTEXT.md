# ofinbox

Tools for processing the OmniFocus inbox: a keyboard-driven TUI and a phone-facing web app, both speaking the same `omnifocus.Client` operation set.

## Language

**Inbox item**:
An OmniFocus task sitting in the inbox, awaiting a processing decision.
_Avoid_: todo, entry, card (a card is the UI presentation of an item, not the item)

**Processing**:
Making a disposition decision on an inbox item — complete, drop, file, or quick-file — which removes it from the session's queue and counts toward the processed tally.
_Avoid_: triaging, clearing

**Queue**:
The client-side working list of inbox items for one session, fetched once and consumed locally as items are processed. Refresh rebuilds it from OmniFocus.
_Avoid_: list, feed

**Filing**:
Moving an inbox item to a project, choosing the destination via a picker. Filing an action group moves the whole subtree intact.
_Avoid_: moving, sorting

**Link item**:
An inbox item whose title or note, trimmed, is exactly one URL. Prose around the URL disqualifies it.
_Avoid_: bookmark, URL task

**Quick-file**:
The one-step disposition for a link item: tag `NoAction` and move to the "Links to Review" project (preferring the one in the "Personal" folder).
_Avoid_: link-file, archive

**Links-only filter**:
A client-side view toggle that hides non-link items from the queue for batch link processing. Non-link items are untouched, only hidden.

**Action group**:
An inbox item with subtasks. Complete and drop cascade to the whole subtree, so those actions require confirmation on a group.
_Avoid_: parent task, project (a group is not yet a project)

**Picker**:
A type-to-filter chooser for projects or tags. Typing a name with no exact match offers a `+ new …` row that creates the destination and applies it in one step.

**Suggestion**:
A project or tag the app recommends for the current item, shown as pinned rows at the top of the picker while the filter is empty. Typing any filter text dismisses suggestions and reverts to plain filtering. A suggestion is advisory: selecting one is the same as selecting that project/tag by search.
_Avoid_: prediction, guess

**Filing history**:
The corpus of past filing decisions used to score suggestions: tasks already living in projects (remaining and completed), with their names and tag assignments. Dropped tasks are not history.

**Recommender**:
The component that ranks projects/tags for an item from its text. The v1 recommender is lexical (token overlap against filing history and project/tag names); the interface exists so a model-backed recommender can replace it without touching the pickers.

**Processed tally**:
The count of items processed this session, shown in the header and as the payoff on the inbox-zero screen. Skipping an item does not count.
