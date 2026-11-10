import { createList, createPageSizeSelect } from '../modules/components.js';
import { isRequestMatch } from '../htmx/helpers.js';

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

/*
Handle any htmx errors that are not swapped by the htmx config.
*/
document.body.addEventListener("htmx:responseError", function(e) {
  // Handle errors for delete user by showing a toast alert.
  if (isRequestMatch(e, "/v1/companies/[0-9a-f-]{36}", "delete")) {
    if (pageAlerts) {
      const error = JSON.parse(e.detail.xhr.response);
      pageAlerts.innerHTML = alertHTML('danger', `Delete Company Error: ${error.error}`);

      let deleteEntryModal = Modal.getInstance(document.getElementById("deleteEntryModal"));
      deleteEntryModal.hide();
    }
    return;
  }
});

/*
When the delete button is pressed in the list, show the modal and populate the modal
contents with the data attributes from the row in the table. When hidden, make sure
the modal is reset to its previous ready state.
*/
const deleteEntryModal = document.getElementById('deleteEntryModal');
if (deleteEntryModal) {
  const deleteBtn = deleteEntryModal.querySelector('#deleteBtn');

  // On show populate the modal contents with data attributes.
  deleteEntryModal.addEventListener('show.bs.modal', function(e) {
    const tgt = e.relatedTarget;
    deleteEntryModal.querySelector('#companyID').textContent = tgt.dataset.bsCompanyId;
    deleteEntryModal.querySelector('#name').textContent = tgt.dataset.bsName;
    deleteEntryModal.querySelector('#website').textContent = tgt.dataset.bsWebsite;
    deleteEntryModal.querySelector('#lei').textContent = tgt.dataset.bsLei;

    deleteBtn.setAttribute("hx-delete", `/v1/companies/${tgt.dataset.bsCompanyId}`);
    htmx.process(deleteBtn);
  });

  // On hide, reset the modal to its previous ready state.
  deleteEntryModal.addEventListener('hidden.bs.modal', function() {
    deleteEntryModal.querySelector('#companyID').textContent = '';
    deleteEntryModal.querySelector('#name').textContent = '';
    deleteEntryModal.querySelector('#website').textContent = '';
    deleteEntryModal.querySelector('#lei').textContent = '';

    deleteBtn.removeAttribute("hx-delete");
    htmx.process(deleteBtn);
  });
}

/*
Post-event handling when the company-deleted event is fired.
*/
document.body.addEventListener("company-deleted", function(e) {
  const elt = e.detail?.elt;
  if (elt) {
    if (elt.id === 'deleteBtn') {
      const deleteEntryModal = Modal.getInstance(document.getElementById("deleteEntryModal"));
      deleteEntryModal.hide();

      if (pageAlerts) {
        pageAlerts.innerHTML = alertHTML('warning', 'Daybreak entry deleted.');
      }
    }
  }
});