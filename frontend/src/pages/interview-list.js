// ── Interview list page ──────────────────────────────────────────────────────

function renderInterviewList() {
    document.getElementById('router-view').innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <h2 class="page-title">Interviews</h2>
            </div>
            <div class="page-header-right">
                <button class="icon-btn" onclick="loadInterviewList()" title="Refresh">↺</button>
                <button class="ghost-btn" onclick="navigate('login')">← Sign Out</button>
            </div>
        </div>
        <div id="interview-list-body" class="table-wrap">
            <div class="table-loading">Loading…</div>
        </div>
    `
    loadInterviewList()
}

async function loadInterviewList() {
    const body = document.getElementById('interview-list-body')
    if (!body) return
    body.innerHTML = '<div class="table-loading">Loading…</div>'

    try {
        const items = await window.go.main.App.ATSInterviewList()

        if (!items || items.length === 0) {
            body.innerHTML = '<div class="table-empty">No interviews found.</div>'
            return
        }

        body.innerHTML = `
            <table class="data-table">
                <thead>
                    <tr>
                        <th>Candidate</th>
                        <th>Round</th>
                        <th>Scheduled</th>
                        <th>Time</th>
                        <th>Status</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    ${items.map(i => `
                    <tr>
                        <td>
                            <div class="cell-primary">${esc(i.candidate_name ?? '')}</div>
                            <div class="cell-secondary">${esc(i.candidate_email ?? '')}</div>
                        </td>
                        <td>${esc(i.interview_round ?? '')}</td>
                        <td>${esc(i.scheduled_on ?? '')}</td>
                        <td class="cell-mono">
                            ${esc((i.from_time ?? '').slice(0, 5))} – ${esc((i.to_time ?? '').slice(0, 5))}
                        </td>
                        <td>
                            <span class="status-badge status-${esc((i.status ?? '').toLowerCase())}">
                                ${esc(i.status ?? '')}
                            </span>
                        </td>
                        <td class="cell-actions">
                            <button class="action-btn" onclick="goToFind('${esc(i.name)}')">View</button>
                            <button class="action-btn action-btn--primary" onclick="goToSession('${esc(i.name)}')">Start</button>
                        </td>
                    </tr>`).join('')}
                </tbody>
            </table>
        `
    } catch (err) {
        body.innerHTML = `<div class="table-error">Failed to load: ${esc(String(err?.message ?? err))}</div>`
    }
}

// Navigate to the interview detail page
function goToFind(name) {
    _selectedInterview = name
    navigate('interview_find')
}

// Navigate directly to the device-select / start-session page
function goToSession(name) {
    _selectedInterview = name
    navigate('start_session')
}
