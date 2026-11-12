import  Alerts from '../modules/alerts.js';
import { createList } from '../modules/components.js';
import { isRequestMatch } from '../htmx/helpers.js';

const pageAlerts = new Alerts(document.querySelector('.pageAlerts'), { dismissible: true, autoClose: true });

document.addEventListener("htmx:afterSettle", function(e) {
  // Only initialize when the contacts list itself has been swapped in.
  if (!e.detail.elt || e.detail.elt.id !== 'contacts') return;

  // The table is only rendered when there are contacts to display.
  const contactList = document.getElementById('contactList');
  if (!contactList || !contactList.querySelector('.list')) return;

  // Initialize List.js (sorting only; no search or pagination).
  createList(contactList);
});

function clearFormErrors(form) {
  form.querySelectorAll('.is-invalid').forEach((el) => {
    el.classList.remove('is-invalid');
    const feedback = el.parentElement.querySelector('.invalid-feedback');
    if (feedback) feedback.textContent = '';
  });
}

// Marks the input matching the API field name as invalid.
function markFieldError(form, field, message) {
  const el = form.querySelector(`[name="${field}"]`);
  if (!el) return false;
  el.classList.add('is-invalid');
  const feedback = el.parentElement.querySelector('.invalid-feedback');
  if (feedback) feedback.textContent = message;
  return true;
}

/*
Create Contact Modal: handle form submission results, showing field level errors on
failure and closing the modal on success (the contacts list refreshes itself on the
contact-created event triggered by the server).
*/
const createContactModal = document.getElementById('createContactModal');
const createContactForm = document.getElementById('createContactForm');
const createContactAlerts = new Alerts(document.getElementById('createContactAlerts'), { dismissible: true, autoClose: true });

if (createContactForm) {
  createContactForm.addEventListener('htmx:afterRequest', function(e) {
    clearFormErrors(createContactForm);
    createContactAlerts.clear();

    if (e.detail.successful) {
      createContactForm.reset();
      Modal.getOrCreateInstance(createContactModal).hide();
      if (pageAlerts) {
        pageAlerts.success('','Contact successfully created.', { dismissible: true, autoClose: true });
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

    const message = (reply && reply.error) || `Request failed with status ${e.detail.xhr.status}`;
    const unmatched = [];
    if (reply && Array.isArray(reply.error_detail)) {
      reply.error_detail.forEach((d) => {
        if (!markFieldError(createContactForm, d.field, d.error)) {
          unmatched.push(d.error);
        }
      });
    }

    if (unmatched.length > 0) {
      body = `<p class="m-0 p-0"><strong>${message}</strong></p><ul class="mb-0 mt-2">${unmatched.map((d) => `<li>${d}</li>`).join('')}</ul>`;
      createContactAlerts.content(body, { level: 'danger', dismissible: true, autoClose: true });
    } else {
      createContactAlerts.danger("Error: ", message);
    }
  });

  // Reset state when the modal is closed.
  createContactModal.addEventListener('hidden.bs.modal', function() {
    clearFormErrors(createContactForm);
    createContactAlerts.clear();
  });
}

/*
Delete Contact Modal: when the delete item is clicked in the list, populate the modal
with the data attributes from the row and arm the delete button. On hide, reset the
modal to its ready state.
*/
const deleteContactModal = document.getElementById('deleteContactModal');
if (deleteContactModal) {
  const deleteBtn = deleteContactModal.querySelector('#deleteContactBtn');

  deleteContactModal.addEventListener('show.bs.modal', function(e) {
    const tgt = e.relatedTarget;
    deleteContactModal.querySelector('#deleteContactName').textContent = tgt.dataset.bsName;
    deleteContactModal.querySelector('#deleteContactEmail').textContent = tgt.dataset.bsEmail;
    deleteContactModal.querySelector('#deleteContactRole').textContent = tgt.dataset.bsRole;

    deleteBtn.setAttribute("hx-delete", `/v1/contacts/${tgt.dataset.bsContactId}`);
    htmx.process(deleteBtn);
  });

  deleteContactModal.addEventListener('hidden.bs.modal', function() {
    deleteContactModal.querySelector('#deleteContactName').textContent = '';
    deleteContactModal.querySelector('#deleteContactEmail').textContent = '';
    deleteContactModal.querySelector('#deleteContactRole').textContent = '';

    deleteBtn.removeAttribute("hx-delete");
    htmx.process(deleteBtn);
  });
}

/*
Post-event handling when the contact-deleted event is fired by the server.
*/
document.body.addEventListener("contact-deleted", function(e) {
  if (e.detail?.elt?.id === 'deleteContactBtn') {
    Modal.getOrCreateInstance(deleteContactModal).hide();
    if (pageAlerts) {
      pageAlerts.warning('', 'Contact successfully deleted.');
    }
  }
});

/*
Handle any htmx errors that are not swapped by the htmx config.
*/
document.body.addEventListener("htmx:responseError", function(e) {
  if (isRequestMatch(e, "/v1/contacts/[0-9a-f-]{36}", "delete")) {
    let message = `Request failed with status ${e.detail.xhr.status}`;
    try {
      const error = JSON.parse(e.detail.xhr.response);
      if (error.error) message = error.error;
    } catch (_) {}

    if (pageAlerts) {
      pageAlerts.danger("Could not delete contact:", message);
    }
    if (deleteContactModal) {
      Modal.getOrCreateInstance(deleteContactModal).hide();
    }
  }
});
