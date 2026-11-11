import  Alerts from '../modules/alerts.js';
import { createList } from '../modules/components.js';
import { isRequestMatch } from '../htmx/helpers.js';

const pageAlerts = new Alerts(document.querySelector('.pageAlerts'), { dismissible: true, autoClose: false });

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

const editCompanyForm = document.getElementById('editCompanyForm');
if (editCompanyForm) {
  editCompanyForm.addEventListener('htmx:configRequest', function(e) {
    if (e.detail.parameters.ivms101) {
      let orig = e.detail.parameters.ivms101;
      try {
        e.detail.parameters.ivms101 = JSON.parse(e.detail.parameters.ivms101);
      } catch (err) {
        console.error('Invalid JSON in ivms101 field:', err);
        e.detail.parameters.ivms101 = orig;
      }
    }
  });

  editCompanyForm.addEventListener('htmx:afterRequest', function(e) {
    clearFormErrors(editCompanyForm);
    pageAlerts.clear();

    if (e.detail.successful) {
      if (pageAlerts) {
        pageAlerts.success('','Company successfully updated.', { dismissible: true, autoClose: true });
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
        if (!markFieldError(editCompanyForm, d.field, d.error)) {
          unmatched.push(d.error);
        }
      });
    }

    if (unmatched.length > 0) {
      body = `<p class="m-0 p-0"><strong>${message}</strong></p><ul class="mb-0 mt-2">${unmatched.map((d) => `<li>${d}</li>`).join('')}</ul>`;
      pageAlerts.content(body, { level: 'danger', dismissible: true, autoClose: true });
    } else {
      pageAlerts.danger("Error: ", message);
    }

    // Scroll to the top of the page.
    window.scrollTo({
      top: 0,
      left: 0,
      behavior: "smooth"
    });
  });
}

/*
Post-event handling when the company-deleted event is fired by the server.
*/
document.body.addEventListener("company-deleted", function(e) {
  window.location.replace("/companies");
});


/*
Handle any htmx errors that are not swapped by the htmx config.
*/
const confirmDeleteModal = document.getElementById('confirmDeleteModal');
document.body.addEventListener("htmx:responseError", function(e) {
  if (isRequestMatch(e, "/v1/companies/[0-9a-f-]{36}", "delete")) {
    let message = `Request failed with status ${e.detail.xhr.status}`;
    try {
      const error = JSON.parse(e.detail.xhr.response);
      if (error.error) message = error.error;
    } catch (_) {}

    if (pageAlerts) {
      pageAlerts.danger("Could not delete company:", message);
    }
    if (confirmDeleteModal) {
      Modal.getOrCreateInstance(confirmDeleteModal).hide();
    }

    // Scroll to the top of the page.
    window.scrollTo({
      top: 0,
      left: 0,
      behavior: "smooth"
    });
  }
});