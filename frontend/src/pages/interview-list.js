// ── Interview list page ──────────────────────────────────────────────────────

function renderInterviewList() {
    document.getElementById('router-view').innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <h2 class="page-title">Interviews (Workable)</h2>
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
        const session = getLoginSession()
        const memberID = session?.member?.id

        if (!memberID) {
            body.innerHTML = '<div class="table-error">No Workable member info found in session. Please sign in again.</div>'
            return
        }

        const items = await window.go.main.App.WorkableInterviewList(memberID)

        if (!items || items.length === 0) {
            body.innerHTML = '<div class="table-empty">No future interviews found in Workable.</div>'
            return
        }

        body.innerHTML = `
            <table class="data-table">
                <thead>
                    <tr>
                        <th>Candidate</th>
                        <th>Job</th>
                        <th>Scheduled</th>
                        <th>Time</th>
                        <th>Type</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    ${items.map(i => {
                        const startsAt = new Date(i.starts_at)
                        const endsAt = new Date(i.ends_at)
                        const dateStr = startsAt.toLocaleDateString()
                        const timeStr = `${startsAt.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'})} – ${endsAt.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'})}`
                        
                        return `
                        <tr>
                            <td>
                                <div class="cell-primary">${esc(i.candidate?.name ?? 'Unknown')}</div>
                            </td>
                            <td>
                                <div class="cell-primary">${esc(i.job?.title ?? 'No Job')}</div>
                            </td>
                            <td>${esc(dateStr)}</td>
                            <td class="cell-mono">${esc(timeStr)}</td>
                            <td>
                                <span class="status-badge status-interview">
                                    ${esc(i.type ?? 'Event')}
                                </span>
                            </td>
                            <td class="cell-actions">
                                <button class="action-btn" onclick="goToFind('${esc(i.id)}')">View</button>
                                <button class="action-btn action-btn--primary" onclick="goToSession('${esc(i.id)}')">Start</button>
                            </td>
                        </tr>`
                    }).join('')}
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
