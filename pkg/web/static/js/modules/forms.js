const clearFormErrors = (form) => {
  form.querySelectorAll('.is-invalid').forEach((el) => {
    el.classList.remove('is-invalid');
    const feedback = el.parentElement.querySelector('.invalid-feedback');
    if (feedback) feedback.textContent = '';
  });
}

const markFieldError = (form, field, message) => {
  const el = form.querySelector(`[name="${field}"]`);
  if (!el) return false;

  el.classList.add('is-invalid');
  const feedback = el.parentElement.querySelector('.invalid-feedback');
  if (feedback) feedback.textContent = message;
  return true;
}

const markFormErrors = (form, e, alerts) => {
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
      if (!markFieldError(form, d.field, d.error)) {
        unmatched.push(d.error);
      }
    });
  }

  if (unmatched.length > 0) {
    body = `<p class="m-0 p-0"><strong>${message}</strong></p><ul class="mb-0 mt-2">${unmatched.map((d) => `<li>${d}</li>`).join('')}</ul>`;
    alerts.content(body, { level: 'danger', dismissible: true, autoClose: true });
  } else {
    alerts.danger("Error: ", message);
  }
}

export {
  clearFormErrors,
  markFieldError,
  markFormErrors,
}