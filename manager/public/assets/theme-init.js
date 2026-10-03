// Apply the saved theme before first paint to avoid a flash. A file of its own (and not an
// inline script) so the Content-Security-Policy can forbid inline scripts.
try {
  var t = localStorage.getItem('whatygo-theme');
  var dark = t ? t === 'dark' : matchMedia('(prefers-color-scheme: dark)').matches;
  if (dark) document.documentElement.classList.add('dark');
} catch (e) {}
