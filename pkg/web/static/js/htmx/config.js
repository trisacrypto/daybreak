// Compares a header to a name case-insensitively.
function hasHeader(headers, name) {
  return Object.keys(headers).some(
    (header) => header.toLowerCase() === name.toLowerCase(),
  );
}

// Apply the application's default response format without overriding a caller's choice.
if (document.body) {
  document.body.addEventListener("htmx:configRequest", (event) => {
    const { headers } = event.detail;

    if (!hasHeader(headers, "Accept")) {
      headers.Accept = "text/html";
    }
  });
}