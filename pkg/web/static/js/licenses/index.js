import  Alerts from '../modules/alerts.js';
import { isRequestMatch } from '../htmx/helpers.js';
import { createList, createPageSizeSelect } from '../modules/components.js';
import { clearFormErrors, markFormErrors } from '../modules/forms.js';

const pageAlerts = new Alerts(document.querySelector('.pageAlerts'), { dismissible: true, autoClose: true });

document.addEventListener("htmx:afterSettle", function(e) {
  // Only initialize when the companies list itself has been swapped in.
  if (!e.detail.elt || e.detail.elt.id !== 'licenses') return;

  // Initialize List.js
  const keyList = document.getElementById('licenseList');
  const list = createList(keyList);

  // Initialize Page Size Select
  const pageSizeSelect = document.getElementById('pageSizeSelect');
  createPageSizeSelect(pageSizeSelect, list);
});

// Create License Form
const createModal = document.getElementById('createLicenseModal');
const createForm = document.getElementById('createLicenseForm');
const createAlerts = new Alerts(document.getElementById('createLicenseAlerts'), { dismissible: true, autoClose: true });

if (createForm) {
  createForm.addEventListener('htmx:afterRequest', function(e) {
    clearFormErrors(createForm);
    createAlerts.clear();

    if (e.detail.successful) {
      createForm.reset();
      Modal.getOrCreateInstance(createModal).hide();
      if (pageAlerts) {
        pageAlerts.success('', 'License successfully created.', { dismissible: true, autoClose: true });
      }
      return;
    }

    // This is an error response so mark the form errors and display the alerts.
    markFormErrors(createForm, e, createAlerts);
  });

  // Reset state when the modal is closed.
  createModal.addEventListener('hidden.bs.modal', function() {
    clearFormErrors(createForm);
    createAlerts.clear();
  });
}

/*
Delete License Modal: when the delete item is clicked in the list, populate the modal
with the data attributes from the row and arm the delete button. On hide, reset the
modal to its ready state.
*/
const deleteLicenseModal = document.getElementById('deleteLicenseModal');
if (deleteLicenseModal) {
  const deleteBtn = deleteLicenseModal.querySelector('#deleteLicenseBtn');

  deleteLicenseModal.addEventListener('show.bs.modal', function(e) {
    const tgt = e.relatedTarget;

    deleteLicenseModal.querySelector('#deleteLicenseName').textContent = tgt.dataset.bsName;
    deleteLicenseModal.querySelector('#deleteLicenseEmail').textContent = tgt.dataset.bsEmail;
    deleteLicenseModal.querySelector('#deleteLicenseDescription').textContent = tgt.dataset.bsDescription;

    deleteBtn.setAttribute("hx-delete", `/v1/licenses/${tgt.dataset.bsLicenseId}`);
    htmx.process(deleteBtn);
  });

  deleteLicenseModal.addEventListener('hidden.bs.modal', function() {
    deleteLicenseModal.querySelector('#deleteLicenseName').textContent = '';

    deleteLicenseModal.querySelector('#deleteLicenseEmail').textContent = '';
    deleteBtn.removeAttribute("hx-delete");
    htmx.process(deleteBtn);
  });
}

/*
Post-event handling when the license-deleted event is fired by the server.
*/
document.body.addEventListener("license-deleted", function(e) {
  if (e.detail?.elt?.id === 'deleteLicenseBtn') {
    Modal.getOrCreateInstance(deleteLicenseModal).hide();
    if (pageAlerts) {
      pageAlerts.warning('', 'License successfully deleted.');
    }
  }
});

/*
Handle any htmx errors that are not swapped by the htmx config.
*/
document.body.addEventListener("htmx:responseError", function(e) {
  // Handle errors for delete user by showing a toast alert.
  // cSpell:ignore HJKMNP
  if (isRequestMatch(e, "/v1/licenses/[0-9A-HJKMNP-TV-Z]{26}", "delete")) {
    let message = `Request failed with status ${e.detail.xhr.status}`;
    try {
      const error = JSON.parse(e.detail.xhr.response);
      if (error.error) message = error.error;
    } catch (_) {}

    if (pageAlerts) {
      pageAlerts.danger("Could not delete license:", message);
    }

    if (deleteLicenseModal) {
      Modal.getOrCreateInstance(deleteLicenseModal).hide();
    }
  }
});