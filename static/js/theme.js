// Shared theme system for ClassGo
// Manages system/light/dark theme with localStorage persistence

(function() {
    var STORAGE_KEY = 'classgo-theme';
    var OLD_KEY = 'admin-theme';

    // One-time migration from old key
    if (localStorage.getItem(OLD_KEY)) {
        localStorage.setItem(STORAGE_KEY, localStorage.getItem(OLD_KEY));
        localStorage.removeItem(OLD_KEY);
    }

    var themeOrder = ['system', 'light', 'dark'];

    var themeIcons = {
        system: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/></svg>',
        light: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"/></svg>',
        dark: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"/></svg>'
    };

    var themeLabels = { system: 'System', light: 'Light', dark: 'Dark' };

    function getTheme() {
        return localStorage.getItem(STORAGE_KEY) || 'system';
    }

    function applyTheme(theme) {
        var isDark = theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
        document.documentElement.classList.toggle('dark', isDark);
        var iconEl = document.getElementById('theme-icon');
        var labelEl = document.getElementById('theme-label');
        if (iconEl) iconEl.innerHTML = themeIcons[theme];
        if (labelEl) labelEl.textContent = themeLabels[theme];
    }

    function cycleTheme() {
        var current = getTheme();
        var next = themeOrder[(themeOrder.indexOf(current) + 1) % themeOrder.length];
        localStorage.setItem(STORAGE_KEY, next);
        applyTheme(next);
    }

    // Listen for OS theme changes when in system mode
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function() {
        if (getTheme() === 'system') applyTheme('system');
    });

    // Initialize theme UI on DOM ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() { applyTheme(getTheme()); });
    } else {
        applyTheme(getTheme());
    }

    // Expose globally
    window.cycleTheme = cycleTheme;
    window.applyTheme = applyTheme;
    window.getTheme = getTheme;
})();
