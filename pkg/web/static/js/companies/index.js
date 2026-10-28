import { createList, createPageSizeSelect } from '../modules/components.js';

document.addEventListener("htmx:afterSettle", function(e) {
  // Only initialize when the companies list itself has been swapped in.
  if (!e.detail.elt || e.detail.elt.id !== 'companies') return;

  // Initialize List.js
  const cpList = document.getElementById('companyList');
  const list = createList(cpList);

  // Initialize Page Size Select
  const pageSizeSelect = document.getElementById('pageSizeSelect');
  createPageSizeSelect(pageSizeSelect, list);
});

// Create Entry Form
const createModal = document.getElementById('createEntryModal');
const createForm = document.getElementById('createEntryForm');
const createAlerts = document.getElementById('createEntryAlerts');
const pageAlerts = document.querySelector('.pageAlerts');

function alertHTML(kind, message, details) {
  let html = `<div class="alert alert-${kind} alert-dismissible fade show" role="alert">${message}`;
  if (details && details.length > 0) {
    html += '<ul class="mb-0 mt-2">';
    details.forEach((d) => { html += `<li>${d}</li>`; });
    html += '</ul>';
  }
  html += '<button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button></div>';
  return html;
}

function clearFormErrors(form) {
  form.querySelectorAll('.is-invalid').forEach((el) => {
    el.classList.remove('is-invalid');
    const feedback = el.parentElement.querySelector('.invalid-feedback');
    if (feedback) feedback.textContent = '';
  });
}

// Marks the input matching the API field name as invalid; array fields such as
// vasp_categories[0] are mapped to their vasp_categories[] control.
function markFieldError(form, field, message) {
  const name = field.replace(/\[\d+\]$/, '[]');
  const el = form.querySelector(`[name="${name}"]`) || form.querySelector(`[name="${field}"]`);
  if (!el) return false;
  el.classList.add('is-invalid');
  const feedback = el.parentElement.querySelector('.invalid-feedback');
  if (feedback) feedback.textContent = message;
  return true;
}

if (createForm) {
  createForm.addEventListener('htmx:afterRequest', function(e) {
    clearFormErrors(createForm);
    createAlerts.innerHTML = '';

    if (e.detail.successful) {
      createForm.reset();
      createModal.querySelector('[data-bs-dismiss="modal"]').click();
      if (pageAlerts) {
        pageAlerts.innerHTML = alertHTML('success', 'Daybreak entry created.');
      }
      return;
    }

    // Parse the JSON error reply returned for HTMX requests.
    let reply = null;
    try {
      reply = JSON.parse(e.detail.xhr.responseText);
    } catch (_) {
      reply = null;
    }

    console.log(reply);

    const message = (reply && reply.error) || `Request failed with status ${e.detail.xhr.status}`;
    const unmatched = [];
    if (reply && Array.isArray(reply.error_detail)) {
      reply.error_detail.forEach((d) => {
        if (!markFieldError(createForm, d.field, d.error)) {
          unmatched.push(d.error);
        }
      });
    }
    createAlerts.innerHTML = alertHTML('danger', message, unmatched);
  });

  // Reset state when the modal is closed.
  createModal.addEventListener('hidden.bs.modal', function() {
    clearFormErrors(createForm);
    createAlerts.innerHTML = '';
  });
}
