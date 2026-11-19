import Alerts from '../modules/alerts.js';

/*
Credentials Modal: the modal body fetches the credentials partial via htmx each time the
modal is shown (see hx-trigger on #credentialsModalBody). On hide, any in-flight request
is aborted and the body is reset to its loading state so that the client secret does not
remain in the DOM. Request failures are rendered in the modal body.
*/
const credentialsModal = document.getElementById('credentialsModal');
const credentialsBody = document.getElementById('credentialsModalBody');

if (credentialsModal && credentialsBody) {
  const loading = credentialsBody.innerHTML;
  const credentialsAlerts = new Alerts(credentialsBody, { dismissible: false });

  /*
  Shows a "Copied!" tooltip over the input for 2 seconds. The tooltip is triggered
  manually so that it is not also shown when the input is hovered or focused, and it is
  not animated so that showing and hiding are synchronous: Bootstrap hides a tooltip
  that is shown again while visible and throws if one is disposed mid-transition.
  */
  const copiedTimers = new Map();

  function hideCopied(input) {
    clearTimeout(copiedTimers.get(input));
    copiedTimers.delete(input);

    const tooltip = Tooltip.getInstance(input);
    tooltip.hide();
    tooltip.dispose();
  }

  function showCopied(input) {
    if (!input.isConnected) return;
    if (copiedTimers.has(input)) hideCopied(input);

    new Tooltip(input, { title: 'Copied!', trigger: 'manual', animation: false }).show();
    copiedTimers.set(input, setTimeout(() => hideCopied(input), 2000));
  }

  credentialsModal.addEventListener('hidden.bs.modal', function() {
    htmx.trigger(credentialsBody, 'htmx:abort');

    // Dispose any visible tooltips before their inputs are removed from the DOM.
    copiedTimers.forEach((_, input) => hideCopied(input));
    credentialsBody.innerHTML = loading;
  });

  credentialsBody.addEventListener('htmx:responseError', function(e) {
    let message = `Request failed with status ${e.detail.xhr.status}`;
    try {
      const error = JSON.parse(e.detail.xhr.response);
      if (error.error) message = error.error;
    } catch (_) {}

    credentialsAlerts.clear();
    credentialsAlerts.danger("Could not load credentials:", message);
  });

  credentialsBody.addEventListener('htmx:sendError', function() {
    credentialsAlerts.clear();
    credentialsAlerts.danger("Could not load credentials:", "the server could not be reached.");
  });

  credentialsBody.addEventListener('htmx:afterSettle', function() {
    const copyClientIDBtn = document.getElementById('copyClientIDBtn');
    const clientID = document.getElementById('clientID');
    const copyClientSecretBtn = document.getElementById('copyClientSecretBtn');
    const clientSecret = document.getElementById('clientSecret');

    if (copyClientIDBtn) {
      copyClientIDBtn.addEventListener('click', function() {
        navigator.clipboard.writeText(clientID.value).then(() => showCopied(clientID));
      });
    }

    if (copyClientSecretBtn) {
      copyClientSecretBtn.addEventListener('click', function() {
        navigator.clipboard.writeText(clientSecret.value).then(() => showCopied(clientSecret));
      });
    }
  });
}
