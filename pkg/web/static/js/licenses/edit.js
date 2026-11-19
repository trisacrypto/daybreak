import  Alerts from '../modules/alerts.js';

const pageAlerts = new Alerts(document.querySelector('.pageAlerts'), { dismissible: true, autoClose: true });

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

const editLicenseForm = document.getElementById('editLicenseForm');
if (editLicenseForm) {
  editLicenseForm.addEventListener('htmx:afterRequest', function(e) {
    clearFormErrors(editLicenseForm);
    pageAlerts.clear();

    if (e.detail.successful) {
      if (pageAlerts) {
        pageAlerts.success('','License successfully updated.', { dismissible: true, autoClose: true });
      }

      // Scroll to the top of the page.
      window.scrollTo({
        top: 0,
        left: 0,
        behavior: "smooth"
      });

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
        if (!markFieldError(editLicenseForm, d.field, d.error)) {
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
document.body.addEventListener("license-deleted", function(e) {
  window.location.replace("/licenses");
});

/*
Handle any htmx errors that are not swapped by the htmx config.
*/
const confirmDeleteModal = document.getElementById('confirmDeleteModal');
if (confirmDeleteModal) {
  confirmDeleteModal.addEventListener("htmx:responseError", function(e) {
      let message = `Request failed with status ${e.detail.xhr.status}`;
      try {
        const error = JSON.parse(e.detail.xhr.response);
        if (error.error) message = error.error;
      } catch (_) {}

      if (pageAlerts) {
        pageAlerts.danger("Could not delete company:", message);
      }

      // Hide modal and scroll to the top of the page.
      Modal.getOrCreateInstance(confirmDeleteModal).hide();
      window.scrollTo({
        top: 0,
        left: 0,
        behavior: "smooth"
      });
  });
}