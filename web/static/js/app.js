// Tabidachi — app.js
(function () {
  'use strict';

  // ============================================================
  // Theme toggle (light / dark)
  // ============================================================
  window.toggleTheme = function () {
    var isDark = document.documentElement.classList.contains('wa-dark');
    if (isDark) {
      document.documentElement.classList.remove('wa-dark');
      localStorage.setItem('tabidachi-theme', 'light');
    } else {
      document.documentElement.classList.add('wa-dark');
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
        btn.innerHTML = '<wa-icon name="check"></wa-icon> Copied!';
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
      if (dlg) dlg.show();
    }
    return false;
  };

  window.closeDataDialog = function (el) {
    var id = el.getAttribute('data-dialog');
    if (id) {
      var dlg = document.getElementById(id);
      if (dlg) dlg.hide();
    }
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
  // HTMX hooks
  // ============================================================
  document.addEventListener('DOMContentLoaded', function () {
    initSortableEvents();
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
      var csrfEl = document.getElementById('csrf-live');
      if (!csrfEl || !csrfEl.value) return;
      var hiddenInput = form.querySelector('input[name="gorilla.csrf.Token"]');
      if (hiddenInput) {
        hiddenInput.value = csrfEl.value;
      }
    });

    document.body.addEventListener('htmx:afterSwap', function () {
      if (document.getElementById('timeline')) {
        scrollToToday();
      }
      initSortableEvents();
    });
  });
})();
