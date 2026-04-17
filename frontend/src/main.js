import { getLoginSession, saveLoginSession, clearLoginSession, hasValidSession } from './utils.js'

// ── State ──────────────────────────────────────────────────────────────────
let _selectedInterview = null   // name of the interview chosen from the list
let recording = false
let srtLines = []
let sigLines = [`# Signals — ${new Date().toISOString()}`, '']
let selectedMic = null
let selectedSpeaker = null
let selectedScreen = null
let historyMode = false
let summaryViewMode = false  // toggle between conversation and summary in history mode
let transcriptHistory = []
let currentPartial = { label: '', text: '' }
let currentSummary = null  // cache the latest summary

const NQI_CONTEXT_LINES = 10

// ── Wails event listeners ──────────────────────────────────────────────────
window.runtime.EventsOn('status', (s) => {
    if (s === 'recording') {
        recording = true
        transcriptHistory = []
        currentPartial = { label: '', text: '' }
        // show history toggle once recording starts
        const ht = document.getElementById('history-toggle')
        if (ht) ht.style.display = 'inline-block'
    } else if (s === 'stopped') {
        recording = false
        _stopRecTimer()
        hidePartial()
        // navigate back to list after session ends
        navigate('interview_list')
    }
})

window.runtime.EventsOn('transcript', (d) => {
    if (!document.getElementById('tx-area')) return

    if (d.isFinal) {
        transcriptHistory.push({ label: d.label, text: d.text })
        srtLines.push(`${srtLines.filter(l => l.match(/^\d+$/)).length + 1}`)
        srtLines.push(`${msToSRT(d.startMs)} --> ${msToSRT(d.endMs)}`)
        srtLines.push(`[${d.label}] ${d.text}`)
        srtLines.push('')
        currentPartial = { label: '', text: '' }
        if (historyMode) { addLineToHistory(d.label, d.text); hidePartial() }
        else updateLiveDisplay()
    } else {
        currentPartial = { label: d.label, text: d.text }
        if (historyMode) showPartial(d.label, d.text)
        else updateLiveDisplay()
    }
})

window.runtime.EventsOn('error', (msg) => showError(msg))
window.runtime.EventsOn('saved', (msg) => showError('✓ ' + msg))

// Streaming signal chunks from the signaling agent
window.runtime.EventsOn('signal_chunk_detected', (chunk) => {
    _onSignalChunk(chunk)
})

// Streaming NQI chunks from the orchestration subscriber
window.runtime.EventsOn('nqi_chunk_recieved', (chunk) => {
    _onNqiChunk(chunk)
})

// Active question pointer — emitted by Go when the mapper resolves a signal
// or when NQE produces a new question.  Shows question text in the NQI panel.
window.runtime.EventsOn('current_question', (text) => {
    _updateCurrentQuestion(text)
})

// Judgment received from judging agent after each answer evaluation
window.runtime.EventsOn('judgment_received', (data) => {
    _onJudgmentReceived(data)
})

// Interview summary updated after each judgment is saved
window.runtime.EventsOn('interview_summary_updated', (summary) => {
    _onInterviewSummaryUpdated(summary)
})

// ── Router ─────────────────────────────────────────────────────────────────
function navigate(route) {
    window.location.hash = route
}

function router() {
    const hash = window.location.hash.replace(/^#\/?/, '')
    const pages = {
        '':               renderLogin,
        'login':          renderLogin,
        'interview_list': renderInterviewList,
        'interview_find': renderInterviewFind,
        'start_session':  renderStartSession,
        'active_session': renderActiveSession,
    }
    const render = pages[hash] || renderLogin
    document.getElementById('router-view').innerHTML = ''
    render()
}

window.addEventListener('hashchange', router)
window.addEventListener('load', router)

// ── Pages ──────────────────────────────────────────────────────────────────
function renderLogin() {
    // Check for valid session and auto-login
    const session = getLoginSession()
    if (session) {
        autoLogin(session)
        return
    }
    
    document.getElementById('router-view').innerHTML = `
        <div class="page-login">
            <h2 class="page-title">Sign In</h2>
            <p class="page-subtitle">Connect to the ATS to begin</p>
            <form id="login-form" onsubmit="submitLogin(event)" autocomplete="off">
                <div class="field-group">
                    <label class="field-label" for="login-usr">Username</label>
                    <input
                        id="login-usr"
                        class="field-input"
                        type="text"
                        placeholder="Administrator"
                        autocomplete="username"
                        required
                    />
                </div>
                <div class="field-group">
                    <label class="field-label" for="login-pwd">Password</label>
                    <input
                        id="login-pwd"
                        class="field-input"
                        type="password"
                        placeholder="••••••••"
                        autocomplete="current-password"
                        required
                    />
                </div>
                <div class="field-group">
                    <label class="field-checkbox">
                        <input id="remember-me" type="checkbox" checked />
                        <span>Remember me for 24 hours</span>
                    </label>
                </div>
                <div id="login-err" class="login-err" style="display:none"></div>
                <button id="login-btn" class="login-btn" type="submit">Sign In</button>
            </form>
        </div>
    `
    // focus username field after render
    setTimeout(() => document.getElementById('login-usr')?.focus(), 50)
}

async function autoLogin(session) {
    const view = document.getElementById('router-view')
    view.innerHTML = `
        <div class="page-login">
            <h2 class="page-title">Welcome back, ${esc(session.fullName)}</h2>
            <p class="page-subtitle">Restoring your session...</p>
            <div style="text-align: center; margin-top: 2rem;">
                <div class="spinner"></div>
            </div>
        </div>
    `
    
    try {
        await window.go.main.App.ATSLogin(session.username, session.password)
        navigate('interview_list')
    } catch (err) {
        // Session invalid - clear and show login form
        clearLoginSession()
        renderLogin()
        setTimeout(() => {
            const errEl = document.getElementById('login-err')
            if (errEl) {
                errEl.textContent = 'Session expired. Please sign in again.'
                errEl.style.display = 'block'
            }
        }, 100)
    }
}

async function submitLogin(e) {
    e.preventDefault()
    const usr = document.getElementById('login-usr').value.trim()
    const pwd = document.getElementById('login-pwd').value
    const remember = document.getElementById('remember-me')?.checked ?? true
    const btn = document.getElementById('login-btn')
    const errEl = document.getElementById('login-err')

    btn.disabled = true
    btn.textContent = 'Signing in…'
    errEl.style.display = 'none'

    try {
        const resp = await window.go.main.App.ATSLogin(usr, pwd)
        // resp = { message, home_page, full_name }
        
        // Save session to localStorage if remember me is checked
        if (remember) {
            saveLoginSession(usr, pwd, resp)
        }
        
        navigate('interview_list')
    } catch (err) {
        errEl.textContent = err?.message ?? String(err)
        errEl.style.display = 'block'
        btn.disabled = false
        btn.textContent = 'Sign In'
    }
}

function renderInterviewList() {
    document.getElementById('router-view').innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <h2 class="page-title">Interviews</h2>
            </div>
            <div class="page-header-right">
                <button class="icon-btn" onclick="loadInterviewList()" title="Refresh">↺</button>
                <button class="ghost-btn" onclick="logout()">← Sign Out</button>
            </div>
        </div>
        <div id="interview-list-body" class="table-wrap">
            <div class="table-loading">Loading…</div>
        </div>
    `
    loadInterviewList()
}

function logout() {
    clearLoginSession()
    navigate('login')
}

async function loadInterviewList() {
    const body = document.getElementById('interview-list-body')
    if (!body) return
    body.innerHTML = '<div class="table-loading">Loading…</div>'

    const session = getLoginSession()
    const memberID = session?.member?.id
    if (!memberID) {
        body.innerHTML = '<div class="table-error">No Workable member found in session. Please sign out and sign in again.</div>'
        return
    }

    try {
        const items = await window.go.main.App.WorkableInterviewList(memberID)

        if (!items || items.length === 0) {
            body.innerHTML = '<div class="table-empty">No upcoming interviews found.</div>'
            return
        }

        // Batch-check question banks for all interviews in parallel
        const qbankFlags = await Promise.all(
            items.map(i => window.go.main.App.HasQuestionBank(i.id).catch(() => false))
        )

        body.innerHTML = `
            <table class="data-table">
                <thead>
                    <tr>
                        <th>Candidate</th>
                        <th>Job</th>
                        <th>Date</th>
                        <th>Time</th>
                        <th>Type</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    ${items.map((i, idx) => {
                        const startsAt = new Date(i.starts_at)
                        const endsAt   = new Date(i.ends_at)
                        const dateStr  = startsAt.toLocaleDateString()
                        const timeStr  = `${startsAt.toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'})} – ${endsAt.toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'})}`
                        const hasQBank = qbankFlags[idx]
                        return `
                    <tr>
                        <td><div class="cell-primary">${esc(i.candidate?.name ?? '')}</div></td>
                        <td>${esc(i.job?.title ?? '')}</td>
                        <td>${esc(dateStr)}</td>
                        <td class="cell-mono">${esc(timeStr)}</td>
                        <td><span class="status-badge status-interview">${esc(i.type ?? '')}</span></td>
                        <td class="cell-actions">
                            <button class="action-btn" onclick="goToFind('${esc(i.id)}')">View</button>
                            ${hasQBank
                                ? `<button class="action-btn action-btn--primary" onclick="goToSession('${esc(i.id)}')">▶ Start</button>`
                                : `<button class="action-btn" style="opacity:0.45;cursor:not-allowed" title="Generate a question bank first" disabled>▶ Start</button>`
                            }
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

function goToFind(name) {
    _selectedInterview = name
    navigate('interview_find')
}

function goToSession(name) {
    _selectedInterview = name
    navigate('start_session')
}

function renderInterviewFind() {
    const interviewId = _selectedInterview || ''
    document.getElementById('router-view').innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <button class="ghost-btn" onclick="navigate('interview_list')">← Back</button>
                <h2 class="page-title" id="find-title">Interview Detail</h2>
            </div>
            <div class="page-header-right" id="find-actions">
                <span class="table-loading" style="font-size:0.85em">Checking…</span>
            </div>
        </div>
        <div id="interview-find-body" class="find-body">
            <div class="table-loading">Loading…</div>
        </div>
    `
    loadInterviewFind()
}

function _renderFindActions(interviewId, hasQBank) {
    const actions = document.getElementById('find-actions')
    if (!actions) return
    if (hasQBank) {
        actions.innerHTML = `
            <button class="action-btn" id="gen-qbank-btn" onclick="generateQuestionBank('${esc(interviewId)}')">↺ Regenerate Bank</button>
            <button class="action-btn action-btn--primary" onclick="goToSession('${esc(interviewId)}')">▶ Start Session</button>
        `
    } else {
        actions.innerHTML = `
            <button class="action-btn action-btn--primary" id="gen-qbank-btn" onclick="generateQuestionBank('${esc(interviewId)}')">⚡ Generate Question Bank</button>
        `
    }
}

async function loadInterviewFind() {
    const body = document.getElementById('interview-find-body')
    if (!body || !_selectedInterview) {
        if (body) body.innerHTML = '<div class="table-error">No interview selected.</div>'
        return
    }

    const interviewId = _selectedInterview

    try {
        const [d, hasQBank] = await Promise.all([
            window.go.main.App.WorkableEventFind(interviewId),
            window.go.main.App.HasQuestionBank(interviewId).catch(() => false),
        ])

        _renderFindActions(interviewId, hasQBank)

        const candidateName = d.candidate?.name ?? d.event?.candidate?.name ?? 'Interview Detail'
        const titleEl = document.getElementById('find-title')
        if (titleEl) titleEl.textContent = candidateName

        const startsAt  = d.event?.starts_at ? new Date(d.event.starts_at) : null
        const endsAt    = d.event?.ends_at   ? new Date(d.event.ends_at)   : null
        const dateStr   = startsAt ? startsAt.toLocaleDateString() : ''
        const timeStr   = startsAt && endsAt
            ? `${startsAt.toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'})} – ${endsAt.toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'})}`
            : ''
        const status    = d.event?.cancelled ? 'Cancelled' : 'Scheduled'
        const meetURL   = d.event?.conference?.url ?? ''
        const jobLoc    = d.job?.location?.location_str ?? d.job?.location?.city ?? ''

        body.innerHTML = `
            <!-- ── Hero bar ── -->
            <div class="find-hero">
                <div class="hero-left">
                    <div class="hero-name">${esc(candidateName)}</div>
                    <div class="hero-meta">
                        ${d.candidate?.email ? `<span class="hero-meta-item">✉ ${esc(d.candidate.email)}</span>` : ''}
                        ${d.candidate?.phone ? `<span class="hero-meta-item">📞 ${esc(d.candidate.phone)}</span>` : ''}
                    </div>
                </div>
                <div class="hero-chips">
                    <span class="status-badge status-${status.toLowerCase()}">${esc(status)}</span>
                    ${dateStr  ? `<span class="hero-chip">📅 ${esc(dateStr)}</span>` : ''}
                    ${timeStr  ? `<span class="hero-chip">🕐 ${esc(timeStr)}</span>` : ''}
                    ${d.job?.title ? `<span class="hero-chip">💼 ${esc(d.job.title)}</span>` : ''}
                    ${jobLoc ? `<span class="hero-chip">🏢 ${esc(jobLoc)}</span>` : ''}
                    ${meetURL ? `<a class="hero-chip hero-chip--link" href="${esc(meetURL)}" target="_blank">📹 Join Meeting</a>` : ''}
                </div>
            </div>

            <!-- ── Tabs ── -->
            <div class="find-tabs">
                <button class="find-tab active" onclick="switchTab('candidate', this)">Candidate</button>
                <button class="find-tab" onclick="switchTab('job', this)">Job</button>
                <button class="find-tab" onclick="switchTab('questions', this); loadQuestionBankTab('${esc(_selectedInterview)}')">Questions</button>
            </div>

            <!-- ── Tab panels ── -->
            <div class="find-panels">

                <!-- CANDIDATE -->
                <div id="tab-candidate" class="find-panel active">
                    <div class="find-scroll">

                        <!-- Contact & status info block -->
                        <div class="find-section">
                            <div class="section-title">Details</div>
                            <div class="info-block">
                                ${d.candidate?.email    ? `<div class="info-row"><span class="info-label">Email</span><span class="info-value">${esc(d.candidate.email)}</span></div>` : ''}
                                ${d.candidate?.phone    ? `<div class="info-row"><span class="info-label">Phone</span><span class="info-value">${esc(d.candidate.phone)}</span></div>` : ''}
                                ${d.candidate?.location?.location_str ? `<div class="info-row"><span class="info-label">Location</span><span class="info-value">${esc(d.candidate.location.location_str)}</span></div>` : ''}
                                ${d.candidate?.stage    ? `<div class="info-row"><span class="info-label">Stage</span><span class="info-value">${esc(d.candidate.stage)}</span></div>` : ''}
                                ${d.candidate?.disqualified ? `<div class="info-row"><span class="info-label">Status</span><span class="info-value" style="color:var(--red,#e53e3e)">Disqualified — ${esc(d.candidate.disqualification_reason ?? '')}</span></div>` : ''}
                                ${d.candidate?.resume_url ? `<div class="info-row"><span class="info-label">Resume</span><span class="info-value"><a href="${esc(d.candidate.resume_url)}" target="_blank">Download PDF</a></span></div>` : ''}
                            </div>
                        </div>

                        ${d.candidate?.social_profiles?.length ? `
                        <div class="find-section">
                            <div class="section-title">Profiles</div>
                            <div class="skill-tags">
                                ${d.candidate.social_profiles.map(p => `<a class="skill-tag" href="${esc(p.url)}" target="_blank">${esc(p.name)}</a>`).join('')}
                            </div>
                        </div>` : ''}

                        ${d.candidate?.summary ? `
                        <div class="find-section">
                            <div class="section-title">Summary</div>
                            <p class="summary-text">${esc(d.candidate.summary)}</p>
                        </div>` : ''}

                        ${d.candidate?.skills?.length ? `
                        <div class="find-section">
                            <div class="section-title">Skills</div>
                            <div class="skill-tags">
                                ${d.candidate.skills.map(s => `<span class="skill-tag">${esc(typeof s === 'string' ? s : s?.name ?? '')}</span>`).join('')}
                            </div>
                        </div>` : ''}

                        ${d.candidate?.experience_entries?.length ? `
                        <div class="find-section">
                            <div class="section-title">Experience</div>
                            <div class="exp-list">
                                ${d.candidate.experience_entries.map(ex => `
                                <div class="exp-item">
                                    <div class="exp-header">
                                        <span class="exp-role">${esc(ex.title ?? '')}</span>
                                        ${ex.company ? `<span class="exp-dot">·</span><span class="exp-company">${esc(ex.company)}</span>` : ''}
                                        <span class="exp-dates">${esc(ex.start_date ?? '')}${ex.end_date ? ' – ' + esc(ex.end_date) : ex.current ? ' – Present' : ''}</span>
                                    </div>
                                    ${ex.summary ? `<p class="exp-summary">${esc(ex.summary)}</p>` : ''}
                                </div>`).join('')}
                            </div>
                        </div>` : ''}

                        ${d.candidate?.education_entries?.length ? `
                        <div class="find-section">
                            <div class="section-title">Education</div>
                            <div class="edu-list">
                                ${d.candidate.education_entries.map(e => `
                                <div class="edu-item">
                                    ${e.degree        ? `<span class="edu-degree">${esc(e.degree)}</span>` : ''}
                                    ${e.school        ? `<span class="edu-inst">${esc(e.school)}</span>` : ''}
                                    ${e.field_of_study ? `<span class="edu-year">${esc(e.field_of_study)}</span>` : ''}
                                    ${e.end_date      ? `<span class="edu-year cell-mono">${esc(e.end_date)}</span>` : ''}
                                </div>`).join('')}
                            </div>
                        </div>` : ''}

                    </div>
                </div>

                <!-- JOB -->
                <div id="tab-job" class="find-panel">
                    <div class="find-scroll">

                        <div class="find-section">
                            <div class="section-title">Role</div>
                            <div class="info-block">
                                <div class="info-row"><span class="info-label">Title</span><span class="info-value">${esc(d.job?.title ?? '')}</span></div>
                                ${d.job?.department ? `<div class="info-row"><span class="info-label">Department</span><span class="info-value">${esc(d.job.department)}</span></div>` : ''}
                                ${jobLoc ? `<div class="info-row"><span class="info-label">Location</span><span class="info-value">${esc(jobLoc)}</span></div>` : ''}
                                ${d.job?.workplace_type ? `<div class="info-row"><span class="info-label">Workplace</span><span class="info-value">${esc(d.job.workplace_type)}</span></div>` : ''}
                                ${d.job?.employment_type ? `<div class="info-row"><span class="info-label">Type</span><span class="info-value">${esc(d.job.employment_type)}</span></div>` : ''}
                            </div>
                        </div>

                        ${d.job?.description ? `
                        <div class="find-section">
                            <div class="section-title">Description</div>
                            <div class="summary-text">${d.job.description}</div>
                        </div>` : ''}

                        ${d.job?.requirements ? `
                        <div class="find-section">
                            <div class="section-title">Requirements</div>
                            <div class="summary-text">${d.job.requirements}</div>
                        </div>` : ''}

                        ${d.job?.benefits ? `
                        <div class="find-section">
                            <div class="section-title">Benefits</div>
                            <div class="summary-text">${d.job.benefits}</div>
                        </div>` : ''}

                    </div>
                </div>

                <!-- QUESTIONS -->
                <div id="tab-questions" class="find-panel">
                    <div class="find-scroll" id="questions-panel-body">
                        <div class="table-loading">Click the tab to load questions…</div>
                    </div>
                </div>

            </div>
        `
    } catch (err) {
        body.innerHTML = `<div class="table-error">Failed to load: ${esc(String(err?.message ?? err))}</div>`
        const actions = document.getElementById('find-actions')
        if (actions) actions.innerHTML = ''
    }
}

function switchTab(name, btn) {
    document.querySelectorAll('.find-tab').forEach(t => t.classList.remove('active'))
    document.querySelectorAll('.find-panel').forEach(p => p.classList.remove('active'))
    btn.classList.add('active')
    document.getElementById('tab-' + name)?.classList.add('active')
}

async function generateQuestionBank(eventID, userPrompt = '') {
    const body = document.getElementById('interview-find-body')
    const actions = document.getElementById('find-actions')

    if (actions) {
        actions.innerHTML = `<span class="table-loading" style="font-size:0.85em">⏳ Generating question bank…</span>`
    }

    // Overlay on top of the existing body — preserves the tab DOM so we can switch to it after
    let overlay = null
    if (body) {
        body.style.position = 'relative'
        overlay = document.createElement('div')
        overlay.style.cssText = 'position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:16px;color:var(--text-muted,#888);background:var(--bg,#111);z-index:10;border-radius:inherit'
        overlay.innerHTML = `
            <div class="spinner"></div>
            <p style="margin:0">AI is generating the question bank — this may take a minute…</p>
        `
        body.appendChild(overlay)
    }

    const removeOverlay = () => {
        overlay?.remove()
        if (body) body.style.position = ''
    }

    try {
        const result = await window.go.main.App.GenerateQuestionBank(eventID, userPrompt)
        removeOverlay()
        if (result === 'ok') {
            _renderFindActions(eventID, true)
            const qTab = document.querySelector('.find-tab:nth-child(3)')
            if (qTab) { switchTab('questions', qTab); loadQuestionBankTab(eventID) }
        } else {
            _renderFindActions(eventID, false)
            showError(result)
        }
    } catch (err) {
        removeOverlay()
        _renderFindActions(eventID, false)
        showError('GenerateQuestionBank: ' + (err?.message ?? String(err)))
    }
}

async function loadQuestionBankTab(eventID) {
    const panel = document.getElementById('questions-panel-body')
    if (!panel) return
    panel.innerHTML = '<div class="table-loading">Loading…</div>'
    try {
        const questions = await window.go.main.App.GetQuestionBank(eventID)
        if (!questions || questions.length === 0) {
            panel.innerHTML = `
                <div class="find-section" style="max-width:560px;padding-top:8px">
                    <div class="section-title" style="margin-bottom:10px">Generate Question Bank</div>
                    <p class="summary-text" style="color:white;margin-bottom:12px">
                        No question bank yet. Optionally add focus instructions for the AI — e.g. <em>"focus on system design"</em> or <em>"include behavioural questions about leadership"</em>.
                    </p>
                    <textarea
                        id="qbank-user-prompt"
                        class="field-input"
                        rows="4"
                        placeholder="Optional: instruct the agent — e.g. focus on backend architecture, skip easy questions, include leadership scenarios…"
                        style="width:100%;resize:vertical;margin-bottom:12px;font-size:0.85rem;line-height:1.5"
                    ></textarea>
                    <button
                        class="action-btn action-btn--primary"
                        onclick="generateQuestionBank('${esc(eventID)}', document.getElementById('qbank-user-prompt')?.value?.trim() ?? '')"
                    >⚡ Generate Question Bank</button>
                </div>`
            return
        }
        panel.innerHTML = questions.map((q, i) => `
            <div class="find-section">
                <div class="section-title">
                    <span class="cell-mono" style="font-size:0.75rem;margin-right:8px">${esc(q.id ?? `Q${i+1}`)}</span>
                    <span class="skill-tag">${esc(q.category ?? '')}</span>
                    <span class="skill-tag">${esc(q.difficulty ?? '')}</span>
                    <span class="skill-tag" style="background:var(--bg3,#2a2a2a)">~${q.estimated_time_minutes ?? '?'} min</span>
                </div>
                <p class="summary-text" style="font-weight:500;margin-bottom:8px">${esc(q.question ?? '')}</p>
                ${q.ideal_answer_keywords ? `<p class="summary-text" style="font-size:0.8rem;color:var(--text-muted,#888)"><em>Keywords:</em> ${esc(q.ideal_answer_keywords)}</p>` : ''}
                ${q.evaluation_criteria?.length ? `
                <div style="margin-top:6px">
                    ${q.evaluation_criteria.map(c => `
                    <div class="info-block" style="margin-bottom:4px">
                        <div class="info-row"><span class="info-label">Must mention</span><span class="info-value">${esc(c.must_mention ?? '')}</span></div>
                        ${c.bonus_points ? `<div class="info-row"><span class="info-label">Bonus</span><span class="info-value">${esc(c.bonus_points)}</span></div>` : ''}
                    </div>`).join('')}
                </div>` : ''}
            </div>`).join('')
    } catch (err) {
        panel.innerHTML = `<div class="table-error">Failed to load: ${esc(String(err?.message ?? err))}</div>`
    }
}

function renderStartSession() {
    document.getElementById('router-view').innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <button class="ghost-btn" onclick="navigate('interview_find')">← Back</button>
                <h2 class="page-title">Select Devices</h2>
            </div>
            <div class="page-header-right">
                <button class="icon-btn" onclick="loadAudioDevices()" title="Refresh devices">↺</button>
            </div>
        </div>

        <div class="device-selector">

            <div class="device-section">
                <h4>🎤 Microphones</h4>
                <div id="mic-list" class="device-list">
                    <div class="loading">Loading devices…</div>
                </div>
            </div>

            <div class="device-section">
                <h4>🔊 Speakers</h4>
                <div id="speaker-list" class="device-list">
                    <div class="loading">Loading devices…</div>
                </div>
            </div>

            <div class="device-section">
                <h4>🖥 Screen <span class="device-section-hint">optional — leave blank to record full desktop</span></h4>
                <div id="screen-list" class="device-list">
                    <div class="loading">Loading screens…</div>
                </div>
            </div>

        </div>

        <div id="rec-bar">
            <button id="rec-btn" onclick="startSessionAndRecord()" disabled>▶ Start Recording</button>
        </div>
    `
    // reset selections so a fresh visit always re-checks
    selectedMic = null
    selectedSpeaker = null
    selectedScreen = null
    loadAudioDevices()
}

function renderActiveSession(timerDeferred = false) {
    document.getElementById('router-view').innerHTML = `
        <!-- ── Top bar: rec controls + timer ── -->
        <div id="rec-bar">
            <span id="rec-dot" class="rec-dot"></span>
            <span id="rec-timer" class="rec-timer">00:00</span>
            <button id="history-toggle" class="ghost-btn" onclick="toggleHistoryMode()" style="display:none">📜 History</button>
            <button id="summary-toggle" class="ghost-btn" onclick="toggleSummaryView()" style="display:none">📊 Summary</button>
            <div class="rec-bar-spacer"></div>
            <button id="rec-btn" onclick="toggleRec()" class="stop-btn">■ Stop</button>
        </div>

        <!-- ── Signal stream bar ── -->
        <div id="sig-bar">
            <span class="sig-lbl">signal</span>
            <span id="sig-stream" class="sig-stream"></span>
            <div id="sig-chips" class="sig-chips"></div>
        </div>

        <!-- ── Main body: transcript left, NQI right ── -->
        <div id="session-body">

            <!-- Transcript -->
            <div id="tx-panel">
                <div id="tx-area">
                    <div id="partial" class="line" style="display:none">
                        <span class="lbl mic" id="p-lbl">Mic</span>
                        <span class="tx partial" id="p-tx"></span>
                    </div>
                </div>
            </div>

            <!-- NQI panel -->
            <div id="nqi-panel">
                <div class="nqi-panel-header">
                    <span class="nqi-panel-title">Questions</span>
                    <button id="eval-answer-btn" class="ghost-btn" onclick="manualEvaluateAnswer()" title="Mark answer complete and evaluate">✦ Ask</button>
                </div>

                <!-- Scrollable content wrapper -->
                <div id="nqi-scroll-wrapper">
                    <!-- Current active question card — updated live as the interview progresses -->
                    <div id="current-question-card" class="current-q-card">
                        <div class="current-q-label">Current Question</div>
                        <div id="current-q-text" class="current-q-text">Waiting for session to start…</div>
                    </div>

                    <!-- Latest Judgment Card -->
                    <div id="judgment-card" class="judgment-card" style="display:none">
                        <div class="judgment-header">
                            <span class="judgment-label">Latest Judgment</span>
                            <span id="judgment-score" class="judgment-score"></span>
                        </div>
                        <div id="judgment-verdict" class="judgment-verdict"></div>
                        <div id="judgment-details" class="judgment-details"></div>
                    </div>

                    <!-- NQI suggestion stream -->
                    <div id="nqi-messages"></div>
                </div>

            </div>

        </div>

        <div id="err-log"></div>
    `


    if (!timerDeferred) _startRecTimer()
    navigate('active_session')
}

// ── Recording timer ────────────────────────────────────────────────────────
let _recTimerInterval = null
let _recStartTime     = null

function _startRecTimer() {
    _recStartTime = Date.now()
    _recTimerInterval = setInterval(() => {
        const el = document.getElementById('rec-timer')
        if (!el) { clearInterval(_recTimerInterval); return }
        const s = Math.floor((Date.now() - _recStartTime) / 1000)
        const mm = String(Math.floor(s / 60)).padStart(2, '0')
        const ss = String(s % 60).padStart(2, '0')
        el.textContent = `${mm}:${ss}`
    }, 1000)
}

function _stopRecTimer() {
    clearInterval(_recTimerInterval)
    _recTimerInterval = null
}

// ── Streaming signal handler ───────────────────────────────────────────────
// signal_chunk_detected fires one text chunk at a time while the signaling
// agent streams. We accumulate into a "current signal" display. When chunks
// stop arriving (debounce), we commit the finished signal as a chip.
let _sigBuf          = ''
let _sigDebounce     = null
const SIG_DEBOUNCE_MS = 1200

function _onSignalChunk(chunk) {
    _sigBuf += chunk
    const el = document.getElementById('sig-stream')
    if (el) el.textContent = _sigBuf

    clearTimeout(_sigDebounce)
    _sigDebounce = setTimeout(() => {
        const signal = _sigBuf.trim()
        _sigBuf = ''
        const stream = document.getElementById('sig-stream')
        if (stream) stream.textContent = ''
        if (!signal || signal === 'UNCLEAR') return
        _commitSignalChip(signal)
    }, SIG_DEBOUNCE_MS)
}

function _commitSignalChip(signal) {
    const container = document.getElementById('sig-chips')
    if (!container) return
    const isQ = signal.startsWith('Q:')
    const chip = document.createElement('span')
    chip.className = `sig-chip ${isQ ? 'question' : 'answer'}`
    chip.title = signal
    chip.textContent = isQ ? `Q` : `A`
    container.appendChild(chip)
}

// ── Streaming NQI handler ──────────────────────────────────────────────────
// nqi_chunk_recieved fires one text chunk at a time. We stream into a bubble
// that is created on the first chunk, then finalised on debounce.
let _nqiBuf          = ''
let _nqiDebounce     = null
let _nqiBubble       = null
const NQI_DEBOUNCE_MS = 1400

function _onNqiChunk(chunk) {
    const messages = document.getElementById('nqi-messages')
    if (!messages) return

    _nqiBuf += chunk

    // Create streaming bubble on first chunk
    if (!_nqiBubble) {
        _nqiBubble = document.createElement('div')
        _nqiBubble.className = 'nqi-bubble nqi-bubble--streaming'
        _nqiBubble.innerHTML = `
            <span class="nqi-label">✦ Next Question <span class="nqi-streaming-dot">…</span></span>
            <div class="nqi-q" id="nqi-stream-text"></div>
        `
        messages.appendChild(_nqiBubble)
        // pulse the ask button
        document.getElementById('infer-btn')?.classList.add('pulse')
    }

    const textEl = document.getElementById('nqi-stream-text')
    if (textEl) textEl.textContent = _nqiBuf
    
    // Scroll the wrapper instead of messages
    const wrapper = document.getElementById('nqi-scroll-wrapper')
    if (wrapper) wrapper.scrollTop = wrapper.scrollHeight

    clearTimeout(_nqiDebounce)
    _nqiDebounce = setTimeout(() => {
        // Finalise bubble
        if (_nqiBubble) {
            _nqiBubble.classList.remove('nqi-bubble--streaming')
            const dot = _nqiBubble.querySelector('.nqi-streaming-dot')
            if (dot) dot.remove()
            // reassign id so future streams get a fresh element
            const textEl = _nqiBubble.querySelector('#nqi-stream-text')
            if (textEl) textEl.removeAttribute('id')
            _nqiBubble = null
        }
        _nqiBuf = ''
        document.getElementById('infer-btn')?.classList.remove('pulse')
    }, NQI_DEBOUNCE_MS)
}

// ── Current question display ──────────────────────────────────────────────
// Called whenever Go emits 'current_question' — on session start (first question)
// and every time the mapper resolves a new active question.
function _updateCurrentQuestion(text) {
    const el = document.getElementById('current-q-text')
    if (!el) return
    // Animate the swap so the recruiter notices the change
    el.classList.add('current-q-text--changing')
    setTimeout(() => {
        el.textContent = text
        el.classList.remove('current-q-text--changing')
    }, 180)
}

// ── Judgment display ──────────────────────────────────────────────────────
// Called when judging agent completes evaluation of a Q&A pair
function _onJudgmentReceived(data) {
    const card = document.getElementById('judgment-card')
    if (!card) return

    const judgment = data.judgment
    const passStatus = judgment.pass ? 'PASS' : 'FAIL'
    const passClass = judgment.pass ? 'pass' : 'fail'

    // Update score badge
    const scoreEl = document.getElementById('judgment-score')
    if (scoreEl) {
        scoreEl.textContent = `${judgment.score}/100 ${passStatus}`
        scoreEl.className = `judgment-score judgment-score--${passClass}`
    }

    // Update verdict
    const verdictEl = document.getElementById('judgment-verdict')
    if (verdictEl) {
        verdictEl.textContent = judgment.verdict || 'No verdict provided'
    }

    // Update details (strengths/weaknesses)
    const detailsEl = document.getElementById('judgment-details')
    if (detailsEl) {
        let html = ''
        
        if (judgment.strengths && judgment.strengths.length > 0) {
            html += '<div class="judgment-section"><span class="judgment-section-title">✓ Strengths:</span><ul>'
            judgment.strengths.forEach(s => {
                html += `<li>${esc(s)}</li>`
            })
            html += '</ul></div>'
        }
        
        if (judgment.weaknesses && judgment.weaknesses.length > 0) {
            html += '<div class="judgment-section"><span class="judgment-section-title">⚠ Weaknesses:</span><ul>'
            judgment.weaknesses.forEach(w => {
                html += `<li>${esc(w)}</li>`
            })
            html += '</ul></div>'
        }
        
        if (judgment.missing_keywords && judgment.missing_keywords.length > 0) {
            html += '<div class="judgment-section"><span class="judgment-section-title">Missing Keywords:</span><div class="keyword-tags">'
            judgment.missing_keywords.forEach(k => {
                html += `<span class="keyword-tag">${esc(k)}</span>`
            })
            html += '</div></div>'
        }
        
        detailsEl.innerHTML = html
    }

    // Show the card with animation
    card.style.display = 'block'
    card.classList.add('judgment-card--new')
    setTimeout(() => card.classList.remove('judgment-card--new'), 500)
}

// ── Interview summary display ──────────────────────────────────────────────
// Called when interview summary is updated after each judgment
function _onInterviewSummaryUpdated(summary) {
    // Cache the summary for use in history mode
    currentSummary = summary
    
    // If we're currently viewing the summary, update it
    if (historyMode && summaryViewMode) {
        showSummaryView()
    }
}

function showSummaryView() {
    const txArea = document.getElementById('tx-area')
    if (!txArea) return
    
    // Clear transcript area
    txArea.innerHTML = ''
    
    if (!currentSummary || !currentSummary.questions || currentSummary.questions.length === 0) {
        txArea.innerHTML = '<div class="summary-placeholder">No questions answered yet...</div>'
        return
    }
    
    // Create summary header
    const answered = currentSummary.questions.filter(q => q.answer && q.answer !== '').length
    const passed = currentSummary.questions.filter(q => q.judgment && q.judgment.pass).length
    
    const header = document.createElement('div')
    header.className = 'summary-header'
    header.innerHTML = `
        <div class="summary-title">Interview Summary</div>
        <div class="summary-stats">${answered} answered • ${passed} passed</div>
    `
    txArea.appendChild(header)
    
    // Render question summaries
    const summaryList = document.createElement('div')
    summaryList.className = 'summary-list'
    
    currentSummary.questions.forEach((qa, idx) => {
        const hasAnswer = qa.answer && qa.answer !== ''
        const hasJudgment = qa.judgment && qa.judgment.score !== undefined
        
        const item = document.createElement('div')
        item.className = 'summary-item'
        
        let html = `<div class="summary-item-header">`
        html += `<span class="summary-item-num">${idx + 1}</span>`
        
        if (hasJudgment) {
            const passClass = qa.judgment.pass ? 'pass' : 'fail'
            html += `<span class="summary-score summary-score--${passClass}">${qa.judgment.score}/100</span>`
            html += `<span class="summary-badge summary-badge--${passClass}">${qa.judgment.pass ? 'PASS' : 'FAIL'}</span>`
        } else if (hasAnswer) {
            html += `<span class="summary-badge summary-badge--pending">Evaluating...</span>`
        } else {
            html += `<span class="summary-badge summary-badge--waiting">Waiting</span>`
        }
        
        html += '</div>'
        html += `<div class="summary-item-question">${esc(qa.question.question || '')}</div>`
        
        if (hasAnswer) {
            html += `<div class="summary-item-answer">${esc(qa.answer.substring(0, 150))}${qa.answer.length > 150 ? '...' : ''}</div>`
        }
        
        if (hasJudgment && qa.judgment.verdict) {
            html += `<div class="summary-item-verdict">${esc(qa.judgment.verdict)}</div>`
        }
        
        item.innerHTML = html
        summaryList.appendChild(item)
    })
    
    txArea.appendChild(summaryList)
    txArea.scrollTop = txArea.scrollHeight
}

// ── Session start (start_session → active_session) ─────────────────────────
async function startSessionAndRecord() {
    if (!selectedMic || !selectedSpeaker) {
        alert('Please select both microphone and speaker')
        return
    }
    if (!_selectedInterview) {
        alert('No interview selected — please go back and choose an interview')
        return
    }

    try {
        // Step 0 — Check Google Drive authorization
        const authResponse = await window.go.main.App.ATSCheckGoogleDriveAuthorization()
        if (authResponse.status === "unauthorized") {
            // Show custom modal or dialog
            const wantsAuth = confirm("Google Drive access is required to upload the interview data. Open the authorization page now?\n\nAfter authorizing, click 'Start Recording' again.")
            if (wantsAuth && authResponse.auth_url) {
                window.runtime.BrowserOpenURL(authResponse.auth_url)
            }
            return
        }

        // Render the active session shell; timer is held until recording is live
        renderActiveSession(/* timerDeferred = */ true)

        const timerEl = document.getElementById('rec-timer')
        if (timerEl) timerEl.textContent = 'Loading…'

        // 1. Join cached question bank with Workable event data and seed all agent sessions
        const beginResult = await window.go.main.App.BeginSession(_selectedInterview)
        if (beginResult !== 'ok') {
            showError('Session init failed: ' + beginResult)
            navigate('start_session')
            return
        }

        // 2. Start audio capture + STT stream
        const recResult = await window.go.main.App.StartRecording(
            selectedMic, selectedSpeaker, selectedScreen ?? ''
        )
        if (recResult !== 'ok') {
            showError('Recording failed: ' + recResult)
            navigate('start_session')
            return
        }

        // 3. Recording confirmed — start the visual timer
        _startRecTimer()

    } catch (err) {
        showError('Unexpected error: ' + err)
        navigate('start_session')
    }
}

// ── Controls ───────────────────────────────────────────────────────────────
function toggleHistoryMode() {
    historyMode = !historyMode
    const btn = document.getElementById('history-toggle')
    const summaryBtn = document.getElementById('summary-toggle')
    const txArea = document.getElementById('tx-area')
    
    if (historyMode) {
        btn.textContent = '🔴 Live'
        btn.classList.add('active')
        summaryBtn.style.display = 'inline-block'  // show summary toggle in history mode
        txArea.classList.add('history-mode')
        
        // Start with conversation view by default
        summaryViewMode = false
        rebuildHistoryView()
    } else {
        btn.textContent = '📜 History'
        btn.classList.remove('active')
        summaryBtn.style.display = 'none'  // hide summary toggle in live mode
        summaryBtn.classList.remove('active')
        summaryViewMode = false
        txArea.classList.remove('history-mode')
        updateLiveDisplay()
    }
}

function toggleSummaryView() {
    if (!historyMode) return  // summary toggle only works in history mode
    
    summaryViewMode = !summaryViewMode
    const btn = document.getElementById('summary-toggle')
    const txArea = document.getElementById('tx-area')
    
    if (summaryViewMode) {
        btn.textContent = '💬 Conversation'
        btn.classList.add('active')
        showSummaryView()
    } else {
        btn.textContent = '📊 Summary'
        btn.classList.remove('active')
        rebuildHistoryView()
    }
}

function toggleRec() {
    const btn = document.getElementById('rec-btn')
    if (btn) {
        btn.textContent = 'Stopping...'
        btn.disabled = true
    }
    window.go.main.App.StopRecording()
}

function onDeviceSelect(type, device) {
    if (type === 'mic') selectedMic = device
    else if (type === 'speaker') selectedSpeaker = device
    else if (type === 'screen') selectedScreen = device
    document.getElementById('rec-btn').disabled = !(selectedMic && selectedSpeaker)
}

function saveFiles() {
    window.go.main.App.SaveFiles(srtLines.join('\n'), sigLines.join('\n'))
        .then(r => showError(r === '' ? 'saved!' : r))
}

// ── NQI ────────────────────────────────────────────────────────────────────
function manualEvaluateAnswer() {
    const evalBtn = document.getElementById('eval-answer-btn')
    if (!evalBtn) return
    
    evalBtn.disabled = true
    evalBtn.textContent = '⏳ Evaluating…'
    
    window.go.main.App.ManualEvaluateAnswer()
        .then(result => {
            if (result === 'ok') {
                showError('✓ Answer evaluation triggered')
                // Visual feedback
                evalBtn.classList.add('success-flash')
                setTimeout(() => evalBtn.classList.remove('success-flash'), 1000)
            } else {
                showError(`Evaluation failed: ${result}`)
            }
        })
        .catch(err => showError(`Evaluation error: ${err}`))
        .finally(() => {
            evalBtn.disabled = false
            evalBtn.textContent = '✓ Complete'
        })
}

function inferNextQuestion() {
    const inferBtn = document.getElementById('infer-btn')
    const sendBtn = document.getElementById('send-btn')
    const prompt = input.value.trim()
    input.value = ''
    const recentTranscript = transcriptHistory
        .slice(-NQI_CONTEXT_LINES)
        .map(item => `${item.label}: ${item.text}`)
        .join('\n')

    inferBtn.disabled = true; sendBtn.disabled = true
    inferBtn.textContent = '⏳…'
    if (prompt) appendUserBubble(prompt)

    window.go.main.App.InferNextQuestion(prompt, recentTranscript)
        .then(result => {
            let parsed
            if (typeof result === 'string' && result !== '') {
                try { parsed = JSON.parse(result) } catch (_) { parsed = null }
            } else if (result && typeof result === 'object') {
                parsed = result
            }
            if (!parsed?.next_question) showError('NQI: no suggestion returned.')
        })
        .catch(err => showError(`NQI error: ${err}`))
        .finally(() => {
            inferBtn.disabled = false; sendBtn.disabled = false
            inferBtn.textContent = '✨ Next Q'
        })
}

// ── UI helpers ─────────────────────────────────────────────────────────────
// Ensure the transcript area has the correct structure for live/history modes
function ensureTranscriptStructure() {
    const area = document.getElementById('tx-area')
    if (!area) return
    
    // Check if partial element exists, if not create it
    let partial = document.getElementById('partial')
    if (!partial) {
        partial = document.createElement('div')
        partial.id = 'partial'
        partial.className = 'line'
        partial.style.display = 'none'
        partial.innerHTML = `
            <span class="lbl mic" id="p-lbl">Mic</span>
            <span class="tx partial" id="p-tx"></span>
        `
        area.appendChild(partial)
    }
}

function updateLiveDisplay() {
    const area = document.getElementById('tx-area')
    if (!area) return
    
    // Ensure structure exists
    ensureTranscriptStructure()
    
    area.querySelectorAll('.line:not(#partial)').forEach(l => l.remove())
    const partial = document.getElementById('partial')
    if (currentPartial.text) {
        document.getElementById('p-lbl').style.display = 'none'
        document.getElementById('p-tx').textContent = currentPartial.text
        partial.style.display = 'flex'; partial.classList.add('live-mode-text')
        area.appendChild(partial)
    } else {
        partial.style.display = 'none'; partial.classList.remove('live-mode-text')
    }
    area.scrollTop = area.scrollHeight
}

function rebuildHistoryView() {
    const area = document.getElementById('tx-area')
    if (!area) return
    
    // Ensure structure exists
    ensureTranscriptStructure()
    
    area.querySelectorAll('.line:not(#partial)').forEach(l => l.remove())
    transcriptHistory.forEach(item => addLineToHistory(item.label, item.text))
    if (currentPartial.text) {
        document.getElementById('p-lbl').style.display = ''
        document.getElementById('partial')?.classList.remove('live-mode-text')
        showPartial(currentPartial.label, currentPartial.text)
    } else {
        hidePartial()
    }
}

function addLineToHistory(label, text) {
    const area = document.getElementById('tx-area')
    if (!area) return
    const div = document.createElement('div')
    div.className = 'line'
    div.innerHTML = `<span class="lbl ${label.toLowerCase()}">${label}</span><span class="tx">${esc(text)}</span>`
    area.insertBefore(div, document.getElementById('partial'))
    const lines = area.querySelectorAll('.line:not(#partial)')
    if (lines.length > 80) lines[0].remove()
    area.scrollTop = area.scrollHeight
}

function showPartial(label, text) {
    const p = document.getElementById('partial')
    if (!p) return
    document.getElementById('p-lbl').textContent = label
    document.getElementById('p-lbl').className = `lbl ${label.toLowerCase()}`
    document.getElementById('p-tx').textContent = text
    p.style.display = 'flex'
    document.getElementById('tx-area').appendChild(p)
    document.getElementById('tx-area').scrollTop = 99999
}

function hidePartial() {
    const p = document.getElementById('partial')
    if (p) p.style.display = 'none'
}

function showError(msg) {
    const log = document.getElementById('err-log')
    if (!log) return
    log.style.display = 'block'
    const line = document.createElement('div')
    line.className = 'err-line'
    const ts = new Date().toLocaleTimeString('en-US', { hour12: false })
    line.textContent = `[${ts}] ${msg}`
    log.appendChild(line)
    log.scrollTop = log.scrollHeight
    const lines = log.querySelectorAll('.err-line')
    if (lines.length > 20) lines[0].remove()
}

function appendUserBubble(prompt) {
    const messages = document.getElementById('nqi-messages')
    if (!messages) return
    const bubble = document.createElement('div')
    bubble.className = 'nqi-user-bubble'
    bubble.innerHTML = `
        <span class="nqi-label">You</span>
        <div class="nqi-user-text">${escapeHtml(prompt)}</div>
    `
    messages.appendChild(bubble)
    
    // Scroll the wrapper instead of messages
    const wrapper = document.getElementById('nqi-scroll-wrapper')
    if (wrapper) wrapper.scrollTop = wrapper.scrollHeight
}

// ── Device loader ──────────────────────────────────────────────────────────
function loadAudioDevices() {
    window.go.main.App.ListAudioDevices()
        .then(devices => {
            // ── Microphones ────────────────────────────────────────────────
            const micList = document.getElementById('mic-list')
            micList.innerHTML = ''
            if (devices.Mics?.length) {
                devices.Mics.forEach((mic, idx) => {
                    const label = document.createElement('label')
                    label.className = 'device-option'
                    const radio = document.createElement('input')
                    radio.type = 'radio'; radio.name = 'microphone'; radio.value = mic.Name
                    
                    // Autoselect if default, or if it's the first one and no default is found yet
                    const isDefault = mic.IsDefault || (idx === 0 && !devices.Mics.some(m => m.IsDefault))
                    if (isDefault) {
                        radio.checked = true
                        onDeviceSelect('mic', mic.Name)
                    }
                    
                    radio.addEventListener('change', () => onDeviceSelect('mic', mic.Name))
                    const span = document.createElement('span')
                    span.className = 'device-name'; span.textContent = mic.Name
                    label.appendChild(radio); label.appendChild(span)

                    if (mic.IsDefault) {
                        const defTag = document.createElement('span')
                        defTag.className = 'tag default-tag'
                        defTag.textContent = 'Default'
                        defTag.style.marginLeft = '8px'
                        defTag.style.fontSize = '0.8em'
                        defTag.style.background = '#060606c3'
                        defTag.style.padding = '2px 6px'
                        defTag.style.borderRadius = '4px'
                        label.appendChild(defTag)
                    }
                    
                    micList.appendChild(label)
                })
            } else {
                micList.innerHTML = '<div class="no-devices">No microphones found</div>'
            }

            // ── Speakers ───────────────────────────────────────────────────
            const speakerList = document.getElementById('speaker-list')
            speakerList.innerHTML = ''
            if (devices.Speakers?.length) {
                devices.Speakers.forEach((speaker, idx) => {
                    const label = document.createElement('label')
                    label.className = 'device-option'
                    const radio = document.createElement('input')
                    radio.type = 'radio'; radio.name = 'speaker'; radio.value = speaker.Name
                    
                    // Autoselect if default, or if it's the first one and no default is found yet
                    const isDefault = speaker.IsDefault || (idx === 0 && !devices.Speakers.some(s => s.IsDefault))
                    if (isDefault) {
                        radio.checked = true
                        onDeviceSelect('speaker', speaker.Name)
                    }
                    
                    radio.addEventListener('change', () => onDeviceSelect('speaker', speaker.Name))
                    const span = document.createElement('span')
                    span.className = 'device-name'; span.textContent = speaker.Name
                    label.appendChild(radio); label.appendChild(span)

                    if (speaker.IsDefault) {
                        const defTag = document.createElement('span')
                        defTag.className = 'tag default-tag'
                        defTag.textContent = 'Default'
                        defTag.style.marginLeft = '8px'
                        defTag.style.fontSize = '0.8em'
                        defTag.style.background = '#060606c3'
                        defTag.style.padding = '2px 6px'
                        defTag.style.borderRadius = '4px'
                        label.appendChild(defTag)
                    }

                    if (speaker.IsPersonal === false) {
                        const loudTag = document.createElement('span')
                        loudTag.className = 'tag loud-tag'
                        loudTag.textContent = 'Loud'
                        loudTag.style.marginLeft = '8px'
                        loudTag.style.fontSize = '0.8em'
                        loudTag.style.background = '#fed7d7'
                        loudTag.style.color = '#c53030'
                        loudTag.style.padding = '2px 6px'
                        loudTag.style.borderRadius = '4px'
                        label.appendChild(loudTag)
                    }

                    speakerList.appendChild(label)
                })
            } else {
                speakerList.innerHTML = '<div class="no-devices">No speakers found</div>'
            }

            // ── Screens ────────────────────────────────────────────────────
            const screenList = document.getElementById('screen-list')
            screenList.innerHTML = ''
            const noneLabel = document.createElement('label')
            noneLabel.className = 'device-option screen-option'
            noneLabel.innerHTML = `
                <input type="radio" name="screen" value="" checked>
                <div class="screen-preview">
                    <div class="screen-thumb-placeholder">🖥</div>
                    <div class="screen-info">
                        <span class="device-name">Full desktop</span>
                        <span class="screen-res">all monitors</span>
                    </div>
                </div>
            `
            noneLabel.querySelector('input').addEventListener('change', () => onDeviceSelect('screen', null))
            screenList.appendChild(noneLabel)

            if (devices.Screens?.length) {
                devices.Screens.forEach(screen => {
                    const label = document.createElement('label')
                    label.className = 'device-option screen-option'
                    label.innerHTML = `
                        <input type="radio" name="screen" value="${screen.ID}">
                        <div class="screen-preview">
                            ${screen.Screenshot
                                ? `<img src="${screen.Screenshot}" alt="${screen.Name}" class="screen-thumb" />`
                                : `<div class="screen-thumb-placeholder">🖥</div>`}
                            <div class="screen-info">
                                <span class="device-name">${screen.Name}</span>
                                <span class="screen-res">${screen.Width}×${screen.Height}</span>
                            </div>
                        </div>
                    `
                    label.querySelector('input').addEventListener('change', () => onDeviceSelect('screen', screen.ID))
                    screenList.appendChild(label)
                })
            }
        })
        .catch(err => {
            console.error('Error listing audio devices:', err)
            document.getElementById('mic-list').innerHTML = '<div class="error">Error loading devices</div>'
            document.getElementById('speaker-list').innerHTML = '<div class="error">Error loading devices</div>'
        })
}

// ── Utils ──────────────────────────────────────────────────────────────────
function msToSRT(ms) {
    const h = Math.floor(ms / 3600000)
    const m = Math.floor((ms % 3600000) / 60000)
    const s = Math.floor((ms % 60000) / 1000)
    const f = ms % 1000
    return `${pad(h)}:${pad(m)}:${pad(s)},${pad(f, 3)}`
}
function pad(n, l = 2) { return String(n).padStart(l, '0') }
function esc(t) { return t.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;') }
function escapeHtml(s) { return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;') }
Object.assign(window, {
    submitLogin, navigate, logout, loadInterviewList,
    goToFind, goToSession, switchTab,
    startSessionAndRecord, toggleRec, toggleHistoryMode, toggleSummaryView,
    inferNextQuestion, manualEvaluateAnswer, loadAudioDevices,
    generateQuestionBank, loadQuestionBankTab,
})
