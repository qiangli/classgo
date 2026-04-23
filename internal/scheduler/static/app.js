// Scheduler UI - vendored from gocron-ui, adapted for ClassGo admin integration.

// State
let schedulerJobs = [];
let schedulerWs = null;
let schedulerConnected = false;
let schedulerExpandedSchedules = new Set();
let schedulerInitialized = false;

function getSchedulerApiBase() {
    return '/api/v1/scheduler';
}

function getSchedulerWebSocketUrl() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.host}/api/v1/scheduler/ws`;
}

// Called once when the scheduler section is first navigated to.
function initSchedulerUI() {
    if (schedulerInitialized) return;
    schedulerInitialized = true;
    connectSchedulerWebSocket();
}

// Stop WebSocket when navigating away (optional, saves resources).
function stopSchedulerUI() {
    if (schedulerWs) {
        schedulerWs.close();
        schedulerWs = null;
    }
}

function connectSchedulerWebSocket() {
    schedulerWs = new WebSocket(getSchedulerWebSocketUrl());

    schedulerWs.onopen = () => {
        schedulerConnected = true;
        updateSchedulerConnectionStatus(true);
        hideSchedulerError();
    };

    schedulerWs.onmessage = (event) => {
        try {
            const message = JSON.parse(event.data);
            if (message.type === 'jobs') {
                schedulerJobs = message.data || [];
                renderSchedulerJobs();
            }
        } catch (err) {
            console.error('Failed to parse scheduler WebSocket message:', err);
        }
    };

    schedulerWs.onerror = () => {
        showSchedulerError('Connection error. Retrying...');
    };

    schedulerWs.onclose = () => {
        schedulerConnected = false;
        updateSchedulerConnectionStatus(false);
        setTimeout(connectSchedulerWebSocket, 3000);
    };
}

function updateSchedulerConnectionStatus(connected) {
    const el = document.getElementById('scheduler-connection-status');
    if (!el) return;
    if (connected) {
        el.className = 'inline-flex items-center gap-1 text-xs font-medium px-2 py-1 rounded-full bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300';
        el.innerHTML = '<span class="w-2 h-2 rounded-full bg-green-500 inline-block"></span> Connected';
    } else {
        el.className = 'inline-flex items-center gap-1 text-xs font-medium px-2 py-1 rounded-full bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300';
        el.innerHTML = '<span class="w-2 h-2 rounded-full bg-red-500 inline-block"></span> Disconnected';
    }
}

function showSchedulerError(message) {
    const banner = document.getElementById('scheduler-error-banner');
    const msgEl = document.getElementById('scheduler-error-message');
    if (banner && msgEl) {
        msgEl.textContent = message;
        banner.classList.remove('hidden');
    }
}

function hideSchedulerError() {
    const banner = document.getElementById('scheduler-error-banner');
    if (banner) banner.classList.add('hidden');
}

// API
async function schedulerRunJob(id) {
    const response = await fetch(`${getSchedulerApiBase()}/jobs/${id}/run`, { method: 'POST' });
    if (!response.ok) throw new Error('Failed to run job');
}

async function handleSchedulerRunJob(id, name) {
    if (!await showConfirm(`Run "${name}" now?`)) return;
    try {
        await schedulerRunJob(id);
        hideSchedulerError();
        showToast(`Job "${name}" executed successfully`, 'success');
    } catch (err) {
        showToast(`Job "${name}" failed: ${err.message}`, 'error');
    }
}

// Rendering
function renderSchedulerJobs() {
    const container = document.getElementById('scheduler-jobs-container');
    if (!container) return;

    const countEl = document.getElementById('scheduler-job-count');
    if (countEl) countEl.textContent = schedulerJobs ? schedulerJobs.length : 0;

    // Clean up expanded state for deleted jobs.
    if (schedulerJobs && schedulerJobs.length > 0) {
        const currentIds = new Set(schedulerJobs.map(j => j.id));
        schedulerExpandedSchedules.forEach(id => {
            if (!currentIds.has(id)) schedulerExpandedSchedules.delete(id);
        });
    } else {
        schedulerExpandedSchedules.clear();
    }

    if (!schedulerJobs || schedulerJobs.length === 0) {
        container.innerHTML = `
            <div class="text-center py-12">
                <div class="text-5xl mb-4 opacity-50">&#128336;</div>
                <h3 class="text-lg font-semibold text-gray-700 dark:text-gray-300 mb-2">No Jobs Running</h3>
                <p class="text-gray-500 dark:text-gray-400 text-sm">Jobs are defined in Go code. Once the application starts with scheduled jobs, they will appear here.</p>
            </div>
        `;
        return;
    }

    container.innerHTML = `
        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            ${schedulerJobs.map(job => renderSchedulerJobCard(job)).join('')}
        </div>
    `;
}

function renderSchedulerJobCard(job) {
    const nextRun = job.nextRun ? schedulerFormatDateTime(job.nextRun) : 'Never';
    const lastRun = job.lastRun ? schedulerFormatDateTime(job.lastRun) : 'Never';
    const timeUntil = job.nextRun ? schedulerGetTimeUntil(job.nextRun) : '';
    const isExpanded = schedulerExpandedSchedules.has(job.id);

    return `
        <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-5 hover:shadow-md transition-shadow">
            <div class="flex justify-between items-start mb-3">
                <h4 class="font-semibold text-gray-900 dark:text-gray-100 break-words flex-1 mr-2">${schedulerEscapeHtml(job.name)}</h4>
                <button onclick="handleSchedulerRunJob('${job.id}', '${schedulerEscapeHtml(job.name)}')"
                    class="shrink-0 px-3 py-1.5 bg-green-600 hover:bg-green-700 text-white text-xs font-medium rounded-lg transition-colors"
                    title="Run now">
                    Run
                </button>
            </div>

            ${job.tags && job.tags.length > 0 ? `
                <div class="flex flex-wrap gap-1 mb-3">
                    ${job.tags.map(tag => `<span class="px-2 py-0.5 bg-indigo-50 dark:bg-indigo-900/30 text-indigo-600 dark:text-indigo-400 text-xs rounded-full">${schedulerEscapeHtml(tag)}</span>`).join('')}
                </div>
            ` : ''}

            ${job.schedule ? `
                <div class="mb-3">
                    <span class="text-xs text-gray-500 dark:text-gray-400 uppercase font-semibold tracking-wide">Schedule</span>
                    <div class="mt-1">
                        <span class="inline-block px-2 py-1 bg-indigo-600 text-white text-xs font-semibold rounded">${schedulerEscapeHtml(job.schedule)}</span>
                        ${job.scheduleDetail ? `<span class="ml-2 text-xs text-gray-500 dark:text-gray-400 font-mono">${schedulerEscapeHtml(job.scheduleDetail)}</span>` : ''}
                    </div>
                </div>
            ` : ''}

            <div class="space-y-2 text-sm">
                <div>
                    <span class="text-xs text-gray-500 dark:text-gray-400 uppercase font-semibold tracking-wide">Next Run</span>
                    <div class="text-gray-800 dark:text-gray-200">
                        ${nextRun}
                        ${timeUntil ? `<span class="ml-2 px-2 py-0.5 bg-teal-50 dark:bg-teal-900/30 text-teal-700 dark:text-teal-400 text-xs rounded-full font-medium">${timeUntil}</span>` : ''}
                    </div>
                </div>
                <div>
                    <span class="text-xs text-gray-500 dark:text-gray-400 uppercase font-semibold tracking-wide">Last Run</span>
                    <div class="text-gray-800 dark:text-gray-200">${lastRun}</div>
                </div>
                <div>
                    <span class="text-xs text-gray-500 dark:text-gray-400 uppercase font-semibold tracking-wide">Scheduler</span>
                    <div class="text-gray-800 dark:text-gray-200">${schedulerEscapeHtml(job.schedulerName || 'Default')}</div>
                </div>
                <div>
                    <span class="text-xs text-gray-500 dark:text-gray-400 uppercase font-semibold tracking-wide">Job ID</span>
                    <div class="text-gray-500 dark:text-gray-400 font-mono text-xs break-all">${job.id}</div>
                </div>
            </div>

            ${job.nextRuns && job.nextRuns.length > 0 ? `
                <div class="mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
                    <button onclick="toggleSchedulerSchedule('${job.id}')" class="text-indigo-600 dark:text-indigo-400 text-sm font-medium hover:underline">
                        ${isExpanded ? '&#9660;' : '&#9654;'} Upcoming Runs
                    </button>
                    <div id="scheduler-schedule-${job.id}" class="${isExpanded ? '' : 'hidden'} mt-2 space-y-1 pl-4">
                        ${job.nextRuns.map(run => `<div class="text-sm text-gray-600 dark:text-gray-400">${schedulerFormatDateTime(run)}</div>`).join('')}
                    </div>
                </div>
            ` : ''}
        </div>
    `;
}

function toggleSchedulerSchedule(jobId) {
    const details = document.getElementById(`scheduler-schedule-${jobId}`);
    if (!details) return;
    if (details.classList.contains('hidden')) {
        details.classList.remove('hidden');
        schedulerExpandedSchedules.add(jobId);
    } else {
        details.classList.add('hidden');
        schedulerExpandedSchedules.delete(jobId);
    }
}

// Utilities
function schedulerFormatDateTime(dateStr) {
    if (!dateStr) return 'Never';
    return new Date(dateStr).toLocaleString();
}

function schedulerGetTimeUntil(dateStr) {
    if (!dateStr) return '';
    const diff = new Date(dateStr).getTime() - Date.now();
    if (diff < 0) return 'Overdue';
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);
    if (days > 0) return `in ${days}d ${hours % 24}h`;
    if (hours > 0) return `in ${hours}h ${minutes % 60}m`;
    if (minutes > 0) return `in ${minutes}m`;
    return `in ${seconds}s`;
}

function schedulerEscapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
