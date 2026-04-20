/**
 * Toast notification system — replaces native alert() with styled popups.
 * Usage: showToast('message') or showToast('message', 'error'|'success'|'info')
 */
(function () {
    'use strict';

    var container = null;

    function getContainer() {
        if (container) return container;
        container = document.createElement('div');
        container.id = 'toast-container';
        container.style.cssText =
            'position:fixed;top:1rem;right:1rem;z-index:9999;display:flex;flex-direction:column;gap:0.5rem;pointer-events:none;max-width:24rem;width:calc(100% - 2rem);';
        document.body.appendChild(container);
        return container;
    }

    var icons = {
        error: '<svg width="20" height="20" viewBox="0 0 20 20" fill="none"><circle cx="10" cy="10" r="9" stroke="currentColor" stroke-width="2"/><path d="M7 7l6 6M13 7l-6 6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>',
        success: '<svg width="20" height="20" viewBox="0 0 20 20" fill="none"><circle cx="10" cy="10" r="9" stroke="currentColor" stroke-width="2"/><path d="M6 10l3 3 5-5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        info: '<svg width="20" height="20" viewBox="0 0 20 20" fill="none"><circle cx="10" cy="10" r="9" stroke="currentColor" stroke-width="2"/><path d="M10 9v5M10 6.5v0" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>'
    };

    var themes = {
        error: { bg: '#fef2f2', border: '#fca5a5', text: '#991b1b', icon: '#dc2626', darkBg: '#451a1a', darkBorder: '#7f1d1d', darkText: '#fca5a5', darkIcon: '#f87171' },
        success: { bg: '#f0fdf4', border: '#86efac', text: '#166534', icon: '#22c55e', darkBg: '#14332a', darkBorder: '#166534', darkText: '#86efac', darkIcon: '#4ade80' },
        info: { bg: '#eff6ff', border: '#93c5fd', text: '#1e40af', icon: '#3b82f6', darkBg: '#1e293b', darkBorder: '#1e3a5f', darkText: '#93c5fd', darkIcon: '#60a5fa' }
    };

    function isDark() {
        return document.documentElement.classList.contains('dark');
    }

    function showToast(message, type) {
        type = type || 'error';
        var theme = themes[type] || themes.info;
        var dark = isDark();

        var el = document.createElement('div');
        el.style.cssText =
            'pointer-events:auto;display:flex;align-items:flex-start;gap:0.625rem;padding:0.75rem 1rem;border-radius:0.5rem;border:1px solid;' +
            'box-shadow:0 4px 12px rgba(0,0,0,' + (dark ? '0.4' : '0.1') + ');' +
            'font-size:0.875rem;line-height:1.4;opacity:0;transform:translateX(1rem);transition:all 0.3s ease;' +
            'background:' + (dark ? theme.darkBg : theme.bg) + ';' +
            'border-color:' + (dark ? theme.darkBorder : theme.border) + ';' +
            'color:' + (dark ? theme.darkText : theme.text) + ';';

        var iconSpan = document.createElement('span');
        iconSpan.style.cssText = 'flex-shrink:0;margin-top:1px;color:' + (dark ? theme.darkIcon : theme.icon);
        iconSpan.innerHTML = icons[type] || icons.info;

        var textSpan = document.createElement('span');
        textSpan.style.cssText = 'flex:1;word-break:break-word;';
        textSpan.textContent = message;

        var closeBtn = document.createElement('button');
        closeBtn.style.cssText =
            'flex-shrink:0;background:none;border:none;cursor:pointer;padding:0;margin-left:0.25rem;opacity:0.5;font-size:1.125rem;line-height:1;' +
            'color:' + (dark ? theme.darkText : theme.text) + ';';
        closeBtn.innerHTML = '&times;';
        closeBtn.onclick = function () { dismiss(el); };

        el.appendChild(iconSpan);
        el.appendChild(textSpan);
        el.appendChild(closeBtn);
        getContainer().appendChild(el);

        // Animate in
        requestAnimationFrame(function () {
            requestAnimationFrame(function () {
                el.style.opacity = '1';
                el.style.transform = 'translateX(0)';
            });
        });

        // Auto-dismiss
        var duration = type === 'error' ? 5000 : 3000;
        var timer = setTimeout(function () { dismiss(el); }, duration);
        el.addEventListener('mouseenter', function () { clearTimeout(timer); });
        el.addEventListener('mouseleave', function () { timer = setTimeout(function () { dismiss(el); }, 2000); });
    }

    function dismiss(el) {
        if (el._dismissed) return;
        el._dismissed = true;
        el.style.opacity = '0';
        el.style.transform = 'translateX(1rem)';
        setTimeout(function () { if (el.parentNode) el.parentNode.removeChild(el); }, 300);
    }

    window.showToast = showToast;
})();
