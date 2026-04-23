/**
 * Confirm dialog — replaces native confirm() with a centered styled modal.
 * Usage: if (!await showConfirm('Delete this?')) return;
 */
(function () {
    'use strict';

    function isDark() {
        return document.documentElement.classList.contains('dark');
    }

    function showConfirm(message) {
        return new Promise(function (resolve) {
            var dark = isDark();

            // Backdrop
            var backdrop = document.createElement('div');
            backdrop.style.cssText =
                'position:fixed;inset:0;z-index:9999;display:flex;align-items:center;justify-content:center;' +
                'background:rgba(0,0,0,0.4);opacity:0;transition:opacity 0.15s ease;';

            // Card
            var card = document.createElement('div');
            card.style.cssText =
                'width:100%;max-width:24rem;margin:0 1rem;border-radius:1rem;padding:1.5rem;' +
                'box-shadow:0 20px 60px rgba(0,0,0,' + (dark ? '0.5' : '0.15') + ');' +
                'transform:scale(0.95);transition:transform 0.15s ease;' +
                'background:' + (dark ? '#1f2937' : '#fff') + ';' +
                'color:' + (dark ? '#f3f4f6' : '#111827') + ';';

            // Icon
            var iconWrap = document.createElement('div');
            iconWrap.style.cssText = 'text-align:center;margin-bottom:1rem;';
            iconWrap.innerHTML =
                '<svg width="40" height="40" viewBox="0 0 24 24" fill="none" style="display:inline-block;color:' +
                (dark ? '#fbbf24' : '#f59e0b') + '">' +
                '<path d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" ' +
                'stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';

            // Message
            var msg = document.createElement('p');
            msg.style.cssText = 'text-align:center;font-size:0.9375rem;line-height:1.5;margin-bottom:1.5rem;';
            msg.textContent = message;

            // Buttons row
            var btnRow = document.createElement('div');
            btnRow.style.cssText = 'display:flex;gap:0.75rem;justify-content:center;';

            var cancelBtn = document.createElement('button');
            cancelBtn.textContent = 'Cancel';
            cancelBtn.style.cssText =
                'padding:0.5rem 1.25rem;border-radius:0.5rem;font-size:0.875rem;font-weight:500;cursor:pointer;' +
                'border:1px solid ' + (dark ? '#4b5563' : '#d1d5db') + ';' +
                'background:' + (dark ? '#374151' : '#f9fafb') + ';' +
                'color:' + (dark ? '#d1d5db' : '#374151') + ';';

            var confirmBtn = document.createElement('button');
            confirmBtn.textContent = 'Confirm';
            confirmBtn.style.cssText =
                'padding:0.5rem 1.25rem;border-radius:0.5rem;font-size:0.875rem;font-weight:500;cursor:pointer;' +
                'border:none;background:#dc2626;color:#fff;';

            btnRow.appendChild(cancelBtn);
            btnRow.appendChild(confirmBtn);
            card.appendChild(iconWrap);
            card.appendChild(msg);
            card.appendChild(btnRow);
            backdrop.appendChild(card);
            document.body.appendChild(backdrop);

            // Animate in
            requestAnimationFrame(function () {
                requestAnimationFrame(function () {
                    backdrop.style.opacity = '1';
                    card.style.transform = 'scale(1)';
                });
            });

            function close(result) {
                backdrop.style.opacity = '0';
                card.style.transform = 'scale(0.95)';
                setTimeout(function () {
                    if (backdrop.parentNode) backdrop.parentNode.removeChild(backdrop);
                    resolve(result);
                }, 150);
            }

            cancelBtn.onclick = function () { close(false); };
            confirmBtn.onclick = function () { close(true); };
            backdrop.addEventListener('click', function (e) {
                if (e.target === backdrop) close(false);
            });
            document.addEventListener('keydown', function handler(e) {
                if (e.key === 'Escape') { document.removeEventListener('keydown', handler); close(false); }
                if (e.key === 'Enter') { document.removeEventListener('keydown', handler); close(true); }
            });

            confirmBtn.focus();
        });
    }

    window.showConfirm = showConfirm;
})();
