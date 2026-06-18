(function () {
  'use strict';

  const DEBOUNCE_MS = 150;
  const COLOR_COUNT = 5;

  // ── DOM refs ────────────────────────────────────────────────────────
  const patternEl    = document.getElementById('pattern');
  const testEl       = document.getElementById('test-input');
  const highlightEl  = document.getElementById('highlight-layer');
  const errorEl      = document.getElementById('error-banner');
  const matchListEl  = document.getElementById('match-list');
  const matchCountEl = document.getElementById('match-count');
  const flagEls      = Array.from(document.querySelectorAll('.flags-wrap input[type="checkbox"]'));

  // ── Detect 'd' (hasIndices) flag support once at startup ─────────────
  const SUPPORTS_D_FLAG = (function () {
    try { new RegExp('x', 'gd'); return true; } catch (_) { return false; }
  })();

  // ── State ────────────────────────────────────────────────────────────
  let debounceTimer = null;

  // ── Utilities ────────────────────────────────────────────────────────
  function debounce(fn) {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(fn, DEBOUNCE_MS);
  }

  function escapeHTML(s) {
    return s
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  function buildFlags() {
    let f = 'g';
    flagEls.forEach(el => { if (el.checked) f += el.value; });
    if (SUPPORTS_D_FLAG) f += 'd';
    return f;
  }

  // ── Core: build RegExp, return null on error ──────────────────────────
  function buildRegex(pattern, flags) {
    try {
      const re = new RegExp(pattern, flags);
      hideError();
      return re;
    } catch (e) {
      showError(e.message);
      return null;
    }
  }

  // ── Main test runner ─────────────────────────────────────────────────
  function runTest() {
    const pattern = patternEl.value;
    const text    = testEl.value;

    if (!pattern) {
      hideError();
      clearResults();
      return;
    }

    const flags = buildFlags();
    const re    = buildRegex(pattern, flags);
    if (!re) {
      clearResults();
      return;
    }

    const matches = [...text.matchAll(re)];
    updateHighlight(text, matches);
    updatePanel(matches, re);
    encodeState();
  }

  // ── Highlight overlay ─────────────────────────────────────────────────
  function updateHighlight(text, matches) {
    if (!matches.length) {
      highlightEl.textContent = text;
      syncScroll();
      return;
    }

    let html = '';
    let pos  = 0;

    matches.forEach((m, idx) => {
      const start = m.index;
      const end   = m.index + m[0].length;

      if (start > pos) {
        html += escapeHTML(text.slice(pos, start));
      }

      if (m[0].length === 0) {
        // Zero-length match: thin vertical marker
        html += `<mark class="c${idx % COLOR_COUNT}" style="border-left:2px solid;opacity:.6"></mark>`;
      } else {
        html += `<mark class="c${idx % COLOR_COUNT}">${escapeHTML(m[0])}</mark>`;
      }

      pos = end;
    });

    if (pos < text.length) {
      html += escapeHTML(text.slice(pos));
    }

    highlightEl.innerHTML = html;
    syncScroll();
  }

  function syncScroll() {
    highlightEl.scrollTop  = testEl.scrollTop;
    highlightEl.scrollLeft = testEl.scrollLeft;
  }

  // ── Match panel ───────────────────────────────────────────────────────
  function updatePanel(matches, re) {
    matchCountEl.textContent = matches.length;
    matchListEl.innerHTML = '';

    if (!matches.length) {
      const li = document.createElement('li');
      li.className = 'empty-state';
      li.textContent = 'No matches.';
      matchListEl.appendChild(li);
      return;
    }

    const groupNames = getGroupNames(re.source);

    matches.forEach((m, idx) => {
      const li      = document.createElement('li');
      li.className  = `match-item c${idx % COLOR_COUNT}`;

      const start = m.index;
      const end   = m.index + m[0].length;

      const header       = document.createElement('div');
      header.className   = 'match-header';
      header.innerHTML   =
        `<span class="match-num">Match ${idx + 1}</span>` +
        `<code class="match-text">${m[0].length ? escapeHTML(m[0]) : '<em style="opacity:.45">empty</em>'}</code>` +
        `<span class="match-pos">[${start}, ${end})</span>`;
      li.appendChild(header);

      // Capture groups: indices 1..n
      if (m.length > 1) {
        const ul    = document.createElement('ul');
        ul.className = 'group-list';

        for (let gi = 1; gi < m.length; gi++) {
          const gItem = document.createElement('li');
          const name  = groupNames[gi - 1] || null;
          const label = name
            ? `Group ${gi} <em>(${escapeHTML(name)})</em>`
            : `Group ${gi}`;
          const val = m[gi];

          if (val === undefined) {
            gItem.innerHTML =
              `<span class="group-name">${label}:</span>` +
              `<span class="no-match-note">did not participate</span>`;
          } else {
            const hasPos = SUPPORTS_D_FLAG && m.indices && m.indices[gi];
            const posStr = hasPos ? `[${m.indices[gi][0]}, ${m.indices[gi][1]})` : '';
            gItem.innerHTML =
              `<span class="group-name">${label}:</span>` +
              `<code class="group-text">${escapeHTML(val)}</code>` +
              (posStr ? `<span class="group-pos">${posStr}</span>` : '');
          }

          ul.appendChild(gItem);
        }
        li.appendChild(ul);
      }

      matchListEl.appendChild(li);
    });
  }

  // Parse regex source to return capture group names in declaration order.
  // Returns array of strings (name) or null (unnamed), length = number of
  // capturing groups.
  function getGroupNames(source) {
    const names = [];
    let i = 0;

    while (i < source.length) {
      const ch = source[i];

      if (ch === '\\') {
        // Skip escaped character
        i += 2;
        continue;
      }

      if (ch === '[') {
        // Skip character class content (simplified: handles \] inside)
        i++;
        while (i < source.length) {
          if (source[i] === '\\') { i += 2; continue; }
          if (source[i] === ']') { i++; break; }
          i++;
        }
        continue;
      }

      if (ch === '(' ) {
        if (source[i + 1] === '?') {
          const next2 = source[i + 2];
          const next3 = source[i + 3];

          if (next2 === ':') {
            // (?:...) non-capturing
            i++;
            continue;
          }
          if (next2 === '=' || next2 === '!') {
            // (?=...) or (?!...) lookahead
            i++;
            continue;
          }
          if (next2 === '<' && (next3 === '=' || next3 === '!')) {
            // (?<=...) or (?<!...) lookbehind
            i++;
            continue;
          }
          if (next2 === '<' && next3 && next3 !== '=' && next3 !== '!') {
            // (?<name>...) named capturing group
            const close = source.indexOf('>', i + 3);
            if (close !== -1) {
              names.push(source.slice(i + 3, close));
            } else {
              names.push(null);
            }
            i++;
            continue;
          }
        }
        // Regular capturing group
        names.push(null);
      }

      i++;
    }

    return names;
  }

  // ── Error display ─────────────────────────────────────────────────────
  function showError(msg) {
    errorEl.textContent = msg;
    errorEl.hidden = false;
  }

  function hideError() {
    errorEl.hidden = true;
    errorEl.textContent = '';
  }

  function clearResults() {
    highlightEl.textContent = testEl.value;
    matchCountEl.textContent = '0';
    matchListEl.innerHTML = '';
    syncScroll();
  }

  // ── URL hash state ────────────────────────────────────────────────────
  function encodeState() {
    const activeFlags = flagEls
      .filter(el => el.checked)
      .map(el => el.value)
      .join('');
    const state = { p: patternEl.value, f: activeFlags, t: testEl.value };
    history.replaceState(null, '', '#' + encodeURIComponent(JSON.stringify(state)));
  }

  function decodeState() {
    const hash = location.hash.slice(1);
    if (!hash) return;
    try {
      const state = JSON.parse(decodeURIComponent(hash));
      if (typeof state.p === 'string') patternEl.value = state.p;
      if (typeof state.t === 'string') testEl.value    = state.t;
      if (typeof state.f === 'string') {
        flagEls.forEach(el => { el.checked = state.f.includes(el.value); });
      }
    } catch (_) { /* ignore malformed hash */ }
  }

  // ── Event wiring ──────────────────────────────────────────────────────
  patternEl.addEventListener('input',   () => debounce(runTest));
  testEl.addEventListener('input',      () => debounce(runTest));
  testEl.addEventListener('scroll',     syncScroll);
  flagEls.forEach(el => el.addEventListener('change', () => debounce(runTest)));
  window.addEventListener('hashchange', () => { decodeState(); runTest(); });

  // Sync overlay scroll when textarea is resized
  new ResizeObserver(syncScroll).observe(testEl);

  // ── Init ──────────────────────────────────────────────────────────────
  decodeState();
  runTest();
})();
