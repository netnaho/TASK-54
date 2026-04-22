/**
 * CareOps — minimal HTMX companion script.
 * Loaded after htmx.min.js.  No bundler needed; plain ES2020.
 */

// Dismiss flash messages automatically after 6 seconds
document.addEventListener('DOMContentLoaded', () => {
  const flash = document.querySelector('.flash');
  if (flash) {
    setTimeout(() => {
      flash.style.transition = 'opacity .4s';
      flash.style.opacity = '0';
      setTimeout(() => flash.remove(), 400);
    }, 6000);
  }
});

// HTMX error handler — show a minimal inline alert on HTMX swap failure
document.addEventListener('htmx:responseError', (event) => {
  console.error('[CareOps] HTMX request failed', event.detail);
  const target = event.detail.target;
  if (target) {
    target.innerHTML = '<div class="alert alert-danger" role="alert">Request failed. Please refresh and try again.</div>';
  }
});
