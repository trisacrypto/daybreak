import  Alerts from '../modules/alerts.js';
import { createList, createPageSizeSelect } from '../modules/components.js';
import { isRequestMatch } from '../htmx/helpers.js';
import { clearFormErrors, markFormErrors } from '../modules/forms.js';

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
const pageAlerts = new Alerts(document.querySelector('.pageAlerts'), { dismissible: true, autoClose: true });
const createForm = document.getElementById('createEntryForm');

if (createForm) {
  const createAlerts = new Alerts(document.getElementById('createEntryAlerts'), { dismissible: true, autoClose: true });
  const createModal = document.getElementById('createEntryModal');

  createForm.addEventListener('htmx:afterRequest', function(e) {
    clearFormErrors(createForm);
    createAlerts.clear();

    if (e.detail.successful) {
      createForm.reset();
      Modal.getOrCreateInstance(createModal).hide();
      if (pageAlerts) {
        pageAlerts.success('', 'Daybreak entry created.', { dismissible: true, autoClose: true });
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
        pageAlerts.warning('', 'Daybreak entry deleted.', { dismissible: true, autoClose: true });
      }
    }
  }
});