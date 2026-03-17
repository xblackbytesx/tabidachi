// Hakken — app.js

// ============================================================
// Scroll to today
// ============================================================
function scrollToToday() {
  const today = new Date().toISOString().slice(0, 10); // YYYY-MM-DD
  const el = document.querySelector(`[data-date="${today}"]`);
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }
}

// ============================================================
// Prompt builder: copy to clipboard
// ============================================================
function copyPrompt() {
  const text = document.getElementById('prompt-text');
  if (!text) return;
  navigator.clipboard.writeText(text.textContent).then(() => {
    const btn = document.getElementById('copy-btn');
    if (btn) {
      const orig = btn.innerHTML;
      btn.innerHTML = '<sl-icon name="check2"></sl-icon> Copied!';
      setTimeout(() => { btn.innerHTML = orig; }, 2000);
    }
  }).catch(() => {
    const range = document.createRange();
    range.selectNode(text);
    window.getSelection().removeAllRanges();
    window.getSelection().addRange(range);
  });
}

// ============================================================
// Builder: open/close dialogs via data-dialog attribute
// All onclick handlers in templates use these static functions.
// ============================================================
function openDataDialog(el) {
  const id = el.getAttribute('data-dialog');
  if (id) document.getElementById(id)?.show();
  return false;
}

function closeDataDialog(el) {
  const id = el.getAttribute('data-dialog');
  if (id) document.getElementById(id)?.hide();
}

// ============================================================
// Builder: event type field switching
// ============================================================
function onEventTypeChangeByAttr(select) {
  const formId = select.getAttribute('data-event-form');
  if (!formId) return;
  const form = document.getElementById(formId);
  if (!form) return;
  const selected = select.value;
  form.querySelectorAll('.event-type-fields').forEach(el => {
    el.style.display = el.getAttribute('data-type') === selected ? '' : 'none';
  });
}

// ============================================================
// HTMX hooks
// ============================================================
document.addEventListener('DOMContentLoaded', function () {
  // Inject the freshest CSRF token from #csrf-live into every HTMX request.
  // Listen on document (not body) so the event is always caught regardless
  // of which element triggers the request. Guard against empty values so we
  // don't send an empty header that causes gorilla/csrf to fall through to
  // the (potentially stale) form-field path.
  document.addEventListener('htmx:configRequest', function (evt) {
    var el = document.getElementById('csrf-live');
    if (el && el.value) {
      evt.detail.headers['X-CSRF-Token'] = el.value;
    }
  });

  // Belt-and-suspenders for non-HTMX form submissions: sync the hidden CSRF
  // field with csrf-live just before the form data is sent so the submitted
  // token always matches the current cookie.
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
  });
});
