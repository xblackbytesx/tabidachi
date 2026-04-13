// Tabidachi — app.js
(function () {
  'use strict';

  var CHECK_SVG = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="icon" aria-hidden="true"><path d="M20 6 9 17l-5-5"/></svg>';

  // ============================================================
  // Theme toggle (light / dark)
  // ============================================================
  window.toggleTheme = function () {
    var isDark = document.documentElement.classList.contains('dark');
    if (isDark) {
      document.documentElement.classList.remove('dark');
      localStorage.setItem('tabidachi-theme', 'light');
    } else {
      document.documentElement.classList.add('dark');
      localStorage.removeItem('tabidachi-theme');
    }
  };

  // ============================================================
  // Scroll to today
  // ============================================================
  function scrollToToday() {
    var today = new Date().toISOString().slice(0, 10); // YYYY-MM-DD
    var el = document.querySelector('[data-date="' + today + '"]');
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }

  // ============================================================
  // Prompt builder: copy to clipboard
  // ============================================================
  window.copyPrompt = function () {
    var text = document.getElementById('prompt-text');
    if (!text) return;
    navigator.clipboard.writeText(text.textContent).then(function () {
      var btn = document.getElementById('copy-btn');
      if (btn) {
        var orig = btn.innerHTML;
        btn.innerHTML = CHECK_SVG + ' Copied!';
        setTimeout(function () { btn.innerHTML = orig; }, 2000);
      }
    }).catch(function () {
      var range = document.createRange();
      range.selectNode(text);
      window.getSelection().removeAllRanges();
      window.getSelection().addRange(range);
    });
  };

  // ============================================================
  // Builder: open/close dialogs via data-dialog attribute
  // All onclick handlers in templates use these static functions.
  // ============================================================
  window.openDataDialog = function (el) {
    var id = el.getAttribute('data-dialog');
    if (id) {
      var dlg = document.getElementById(id);
      if (dlg) dlg.showModal();
    }
    return false;
  };

  window.closeDataDialog = function (el) {
    var id = el.getAttribute('data-dialog');
    if (id) {
      var dlg = document.getElementById(id);
      if (dlg) dlg.close();
    }
  };

  // Close modal dialog on backdrop click
  document.addEventListener('click', function (e) {
    if (e.target.tagName === 'DIALOG' && e.target.classList.contains('modal-dialog')) {
      e.target.close();
    }
  });

  // ============================================================
  // Settings: copy token to clipboard
  // ============================================================
  window.copyToken = function () {
    var el = document.getElementById('new-token-value');
    if (!el) return;
    navigator.clipboard.writeText(el.textContent).then(function () {
      var btn = document.getElementById('copy-token-btn');
      if (btn) {
        var orig = btn.innerHTML;
        btn.innerHTML = CHECK_SVG + ' Copied!';
        setTimeout(function () { btn.innerHTML = orig; }, 2000);
      }
    });
  };

  // ============================================================
  // Trip view: copy share link to clipboard
  // ============================================================
  window.copyShareLink = function () {
    var el = document.getElementById('new-share-url');
    if (!el) return;
    var token = el.getAttribute('data-share-token');
    var url = window.location.origin + '/share/' + token;
    navigator.clipboard.writeText(url).then(function () {
      var btn = document.getElementById('copy-share-btn');
      if (btn) {
        var orig = btn.innerHTML;
        btn.innerHTML = CHECK_SVG + ' Copied!';
        setTimeout(function () { btn.innerHTML = orig; }, 2000);
      }
    });
  };

  // ============================================================
  // Builder: inline day edit toggle
  // ============================================================
  window.toggleDayEdit = function (btn) {
    var dayBuilder = btn.closest('.day-builder');
    var form = dayBuilder.querySelector('.day-edit-form');
    form.style.display = form.style.display === 'none' ? 'block' : 'none';
  };

  // ============================================================
  // Trip view: open event photo lightbox
  // ============================================================
  window.openEventLightbox = function (btn) {
    var img = document.getElementById('event-lightbox-img');
    img.src = btn.dataset.fullUrl;
    img.alt = btn.getAttribute('aria-label') || 'Event photo';
    document.getElementById('event-lightbox').showModal();
  };

  // ============================================================
  // Builder: event type field switching
  // ============================================================
  window.onEventTypeChangeByAttr = function (select) {
    var formId = select.getAttribute('data-event-form');
    if (!formId) return;
    var form = document.getElementById(formId);
    if (!form) return;
    var selected = select.value;
    form.querySelectorAll('.event-type-fields').forEach(function (el) {
      el.style.display = el.getAttribute('data-type') === selected ? '' : 'none';
    });
  };

  // ============================================================
  // Builder: sortable event lists (drag-and-drop reordering)
  // ============================================================
  function initSortableEvents() {
    if (typeof Sortable === 'undefined') return;
    document.querySelectorAll('.day-events-preview[data-sortable-url]').forEach(function (container) {
      if (container._sortable) return; // already initialized
      container._sortable = Sortable.create(container, {
        handle: '.drag-handle',
        animation: 150,
        ghostClass: 'sortable-ghost',
        chosenClass: 'sortable-chosen',
        dragClass: 'sortable-drag',
        onEnd: function () {
          var items = container.querySelectorAll('.sortable-item');
          var order = [];
          items.forEach(function (item) {
            order.push(parseInt(item.getAttribute('data-event-idx'), 10));
          });
          var csrfEl = document.getElementById('csrf-live');
          var headers = { 'Content-Type': 'application/json' };
          if (csrfEl && csrfEl.value) {
            headers['X-CSRF-Token'] = csrfEl.value;
          }
          fetch(container.getAttribute('data-sortable-url'), {
            method: 'POST',
            headers: headers,
            body: JSON.stringify(order)
          }).then(function (res) {
            if (!res.ok) {
              console.error('Reorder failed:', res.status);
              window.location.reload();
            } else {
              // Brief flash to confirm save
              container.classList.add('reorder-saved');
              setTimeout(function () { container.classList.remove('reorder-saved'); }, 600);
            }
          }).catch(function () {
            window.location.reload();
          });
        }
      });
    });
  }

  // ============================================================
  // Builder: async event form handling (add/edit/delete events)
  // Forms with .event-async-form are submitted via fetch, the
  // server responds with a fresh DayBuilder fragment, and we
  // swap it into the DOM manually — bypassing HTMX to avoid
  // issues with outerHTML swaps inside open <dialog> elements.
  // ============================================================
  function initAsyncEventForms() {
    document.querySelectorAll('.event-async-form').forEach(function (form) {
      if (form._asyncBound) return;
      form._asyncBound = true;
      form.addEventListener('submit', function (e) {
        e.preventDefault();
        if (form._asyncPending) return; // prevent double-submit
        var dayBuilderId = form.getAttribute('data-day-builder');
        if (!dayBuilderId) return;
        form._asyncPending = true;
        var submitBtn = form.querySelector('button[type="submit"]');
        if (submitBtn) submitBtn.disabled = true;

        // Sync CSRF token
        var csrfEl = document.getElementById('csrf-live');
        var hiddenInput = form.querySelector('input[name="gorilla.csrf.Token"]');
        if (csrfEl && csrfEl.value && hiddenInput) {
          hiddenInput.value = csrfEl.value;
        }

        var headers = { 'X-Day-Refresh': 'true' };
        if (csrfEl && csrfEl.value) {
          headers['X-CSRF-Token'] = csrfEl.value;
        }

        fetch(form.action, {
          method: 'POST',
          headers: headers,
          body: new FormData(form)
        }).then(function (res) {
          if (!res.ok) throw new Error('Save failed: ' + res.status);
          return res.text();
        }).then(function (html) {
          // Close dialog if the form is inside one
          var dlg = form.closest('dialog');
          if (dlg) dlg.close();

          // Replace the day builder with the fresh HTML
          var target = document.getElementById(dayBuilderId);
          if (target) {
            // Capture open <details> before removing them from the DOM
            var openDetails = captureOpenDetails(target);

            // Destroy old Sortable instances before removing DOM
            target.querySelectorAll('.day-events-preview').forEach(function (c) {
              if (c._sortable) { c._sortable.destroy(); c._sortable = null; }
            });
            var tmp = document.createElement('div');
            tmp.innerHTML = html;
            var newBuilder = tmp.querySelector('#' + dayBuilderId) || tmp.firstElementChild;
            if (newBuilder) {
              target.replaceWith(newBuilder);
              initSortableEvents();
              initAsyncEventForms();
              restoreOpenDetails(openDetails);
            }
          }
        }).catch(function (err) {
          console.error('Event save error:', err);
          form._asyncPending = false;
          if (submitBtn) submitBtn.disabled = false;
          window.location.reload();
        });
      });
    });
  }

  // ============================================================
  // View filters (Notes / Alternatives toggles)
  // Persisted in localStorage. Applied via CSS classes on #timeline.
  // ============================================================
  function initViewFilters() {
    var bar = document.getElementById('view-filter-bar');
    if (!bar) return;
    var timeline = document.getElementById('timeline');
    if (!timeline) return;

    var saved = {};
    try { saved = JSON.parse(localStorage.getItem('tabidachi-view-filters') || '{}'); } catch (e) {}

    // true = content visible (default on)
    var state = {
      notes: saved.notes !== false,
      alternatives: saved.alternatives !== false
    };

    function applyFilter(key) {
      timeline.classList.toggle('hide-' + key, !state[key]);
      var btn = document.getElementById('filter-btn-' + key);
      if (btn) btn.classList.toggle('is-active', state[key]);
    }

    function applyAll() {
      applyFilter('notes');
      applyFilter('alternatives');
    }

    ['notes', 'alternatives'].forEach(function (key) {
      var btn = document.getElementById('filter-btn-' + key);
      if (!btn || btn._filterBound) return;
      btn._filterBound = true;
      btn.addEventListener('click', function () {
        state[key] = !state[key];
        localStorage.setItem('tabidachi-view-filters', JSON.stringify(state));
        applyAll();
      });
    });

    applyAll();
  }

  // ============================================================
  // <details> open-state preservation across DOM swaps
  // Captures which <details id="..."> are open before a swap
  // and re-opens them afterwards by ID.
  // ============================================================
  function captureOpenDetails(root) {
    var ids = [];
    root.querySelectorAll('details[open][id]').forEach(function (d) {
      ids.push(d.id);
    });
    return ids;
  }

  function restoreOpenDetails(ids) {
    ids.forEach(function (id) {
      var el = document.getElementById(id);
      if (el) el.open = true;
    });
  }

  // ============================================================
  // View tabs (Timeline / Phrasebook)
  // ============================================================
  function initViewTabs() {
    var tabs = document.querySelectorAll('.view-tab');
    if (!tabs.length) return;

    var timelineView = document.getElementById('view-timeline');
    var phrasebookView = document.getElementById('view-phrasebook');

    function activate(view) {
      tabs.forEach(function (t) {
        t.classList.toggle('active', t.dataset.view === view);
      });
      if (timelineView) timelineView.style.display = view === 'timeline' ? '' : 'none';
      if (phrasebookView) phrasebookView.style.display = view === 'phrasebook' ? '' : 'none';
    }

    tabs.forEach(function (tab) {
      tab.addEventListener('click', function () { activate(tab.dataset.view); });
    });

    activate('timeline');
  }

  // ============================================================
  // JSON / Builder editor tab switcher
  // ============================================================
  function initJsonEditor() {
    var tabs = document.querySelectorAll('.editor-tab');
    if (!tabs.length) return;

    var builderPanel = document.getElementById('editor-builder-panel');
    var jsonPanel = document.getElementById('editor-json-panel');
    var addLegBtn = document.querySelector('.editor-add-leg-btn');
    var STORAGE_KEY = 'tabidachi-editor-mode';

    function activate(mode) {
      tabs.forEach(function (t) {
        t.classList.toggle('active', t.dataset.tab === mode);
      });
      if (builderPanel) builderPanel.style.display = mode === 'builder' ? '' : 'none';
      if (jsonPanel) jsonPanel.style.display = mode === 'json' ? '' : 'none';
      if (addLegBtn) addLegBtn.style.display = mode === 'builder' ? '' : 'none';
      localStorage.setItem(STORAGE_KEY, mode);
    }

    tabs.forEach(function (tab) {
      tab.addEventListener('click', function () { activate(tab.dataset.tab); });
    });

    // If there's a JSON parse/validation error visible, force JSON tab
    var hasJsonError = jsonPanel && jsonPanel.querySelector('.json-editor-error');
    var initial = hasJsonError ? 'json' : (localStorage.getItem(STORAGE_KEY) || 'builder');
    activate(initial);
  }

  // ============================================================
  // HTMX hooks
  // ============================================================
  document.addEventListener('DOMContentLoaded', function () {
    initSortableEvents();
    initAsyncEventForms();
    initViewFilters();
    initViewTabs();
    initJsonEditor();
    if (document.getElementById('timeline')) {
      scrollToToday();
    }
    // Inject the freshest CSRF token from #csrf-live into every HTMX request.
    document.addEventListener('htmx:configRequest', function (evt) {
      var el = document.getElementById('csrf-live');
      if (el && el.value) {
        evt.detail.headers['X-CSRF-Token'] = el.value;
      }
    });

    // Sync the hidden CSRF field with csrf-live just before non-HTMX form submissions.
    document.addEventListener('submit', function (evt) {
      var form = evt.target;
      if (!form || form.tagName !== 'FORM') return;
      if (form.classList.contains('event-async-form')) return; // handled above
      var csrfEl = document.getElementById('csrf-live');
      if (!csrfEl || !csrfEl.value) return;
      var hiddenInput = form.querySelector('input[name="gorilla.csrf.Token"]');
      if (hiddenInput) {
        hiddenInput.value = csrfEl.value;
      }
    });

    // Preserve open <details> elements across HTMX partial swaps.
    var _savedOpenDetails = [];
    document.body.addEventListener('htmx:beforeSwap', function (evt) {
      var target = evt.detail.target;
      if (target) _savedOpenDetails = captureOpenDetails(target);
    });

    document.body.addEventListener('htmx:afterSwap', function () {
      if (document.getElementById('timeline')) {
        scrollToToday();
      }
      initSortableEvents();
      initAsyncEventForms();
      initViewFilters();
      restoreOpenDetails(_savedOpenDetails);
      _savedOpenDetails = [];
    });
  });
})();
