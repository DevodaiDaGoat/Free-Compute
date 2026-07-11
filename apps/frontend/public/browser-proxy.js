(function() {
  function resolveUrl(href) {
    try {
      return new URL(href, document.baseURI).href;
    } catch (e) {
      return href;
    }
  }

  function isExternal(abs) {
    try {
      return new URL(abs).origin !== document.location.origin;
    } catch (e) {
      return false;
    }
  }

  var API_PREFIX = '/api/browser/proxy?url=';

  document.addEventListener('DOMContentLoaded', function() {
    document.querySelectorAll('a[href],area[href]').forEach(function(el) {
      var raw = el.getAttribute('href');
      if (!raw || /^(javascript:|mailto:|tel:|#)/.test(raw)) return;
      var abs = resolveUrl(raw);
      el.addEventListener('click', function(e) {
        if (/^(javascript:|mailto:|tel:|#)/.test(abs)) return;
        e.preventDefault();
        if (isExternal(abs)) {
          window.location.href = API_PREFIX + encodeURIComponent(abs);
        }
      });
    });

    document.querySelectorAll('form').forEach(function(f) {
      f.addEventListener('submit', function(e) {
        var action = f.getAttribute('action') || '';
        if (action && !/^(javascript:|#)/.test(action)) {
          var abs = resolveUrl(action);
          if (isExternal(abs)) {
            e.preventDefault();
            f.setAttribute('action', API_PREFIX + encodeURIComponent(abs));
            if (f.method !== 'get') {
              f.setAttribute('method', 'get');
            }
            f.submit();
          }
        }
      });
    });
  });
})();
