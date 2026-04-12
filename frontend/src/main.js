// ── State ──────────────────────────────────────────────────────────────────
let _selectedInterview = null   // name of the interview chosen from the list
let recording = false
let srtLines = []
let sigLines = [`# Signals — ${new Date().toISOString()}`, '']
let selectedMic = null
let selectedSpeaker = null
let selectedScreen = null
let historyMode = false
let transcriptHistory = []
let currentPartial = { label: '', text: '' }

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
                        <td class="cell-mono">${esc((i.from_time ?? '').slice(0,5))} – ${esc((i.to_time ?? '').slice(0,5))}</td>
                        <td><span class="status-badge status-${(i.status ?? '').toLowerCase()}">${esc(i.status ?? '')}</span></td>
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

function goToFind(name) {
    _selectedInterview = name
    navigate('interview_find')
}

function goToSession(name) {
    _selectedInterview = name
    navigate('start_session')
}

function renderInterviewFind() {
    document.getElementById('router-view').innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <button class="ghost-btn" onclick="navigate('interview_list')">← Back</button>
                <h2 class="page-title" id="find-title">Interview Detail</h2>
            </div>
            <div class="page-header-right">
                <button class="action-btn action-btn--primary" onclick="goToSession(_selectedInterview)">▶ Start Session</button>
            </div>
        </div>
        <div id="interview-find-body" class="find-body">
            <div class="table-loading">Loading…</div>
        </div>
    `
    loadInterviewFind()
}

async function loadInterviewFind() {
    const body = document.getElementById('interview-find-body')
    if (!body || !_selectedInterview) {
        body.innerHTML = '<div class="table-error">No interview selected.</div>'
        return
    }

    try {
        const d = await window.go.main.App.ATSInterviewFind(_selectedInterview)

        // Update header title once we have the name
        const titleEl = document.getElementById('find-title')
        if (titleEl) titleEl.textContent = d.candidate?.name ?? 'Interview Detail'

        body.innerHTML = `
            <!-- ── Sticky hero bar ── -->
            <div class="find-hero">
                <div class="hero-left">
                    <div class="hero-name">${esc(d.candidate?.name ?? '')}</div>
                    <div class="hero-meta">
                        <span class="hero-meta-item">✉ ${esc(d.candidate?.email ?? '')}</span>
                        ${d.candidate?.phone ? `<span class="hero-meta-item">📞 ${esc(d.candidate.phone)}</span>` : ''}
                    </div>
                </div>
                <div class="hero-chips">
                    <span class="status-badge status-${(d.interview?.status ?? '').toLowerCase()}">${esc(d.interview?.status ?? '')}</span>
                    <span class="hero-chip">📅 ${esc(d.interview?.scheduled_on ?? '')}</span>
                    <span class="hero-chip">🎯 ${esc(d.round?.name ?? '')}</span>
                    <span class="hero-chip">💼 ${esc(d.job?.title ?? '')}</span>
                    <span class="hero-chip">🏢 ${esc(d.job?.department ?? '')} · ${esc(d.job?.location ?? '')}</span>
                </div>
            </div>

            <!-- ── Tabs ── -->
            <div class="find-tabs">
                <button class="find-tab active" onclick="switchTab('candidate', this)">Candidate</button>
                <button class="find-tab" onclick="switchTab('job', this)">Job</button>
                <button class="find-tab" onclick="switchTab('questions', this)">
                    Questions
                    <span class="tab-badge">${d.question_bank?.questions?.length ?? 0}</span>
                </button>
            </div>

            <!-- ── Tab panels ── -->
            <div class="find-panels">

                <!-- CANDIDATE -->
                <div id="tab-candidate" class="find-panel active">
                    <div class="find-scroll">

                        <div class="find-section">
                            <div class="section-title">Summary</div>
                            <p class="summary-text">${esc(d.candidate?.summary ?? '')}</p>
                        </div>

                        ${d.candidate?.skills?.length ? `
                        <div class="find-section">
                            <div class="section-title">Skills</div>
                            <div class="skill-tags">
                                ${d.candidate.skills.map(s => `<span class="skill-tag">${esc(s)}</span>`).join('')}
                            </div>
                        </div>` : ''}

                        ${d.candidate?.experience?.length ? `
                        <div class="find-section">
                            <div class="section-title">Experience</div>
                            <div class="exp-list">
                                ${d.candidate.experience.map(ex => `
                                <div class="exp-item">
                                    <div class="exp-header">
                                        <span class="exp-role">${esc(ex.role ?? '')}</span>
                                        <span class="exp-dot">·</span>
                                        <span class="exp-company">${esc(ex.company ?? '')}</span>
                                        <span class="exp-dates">${esc(ex.from ?? '')}${ex.to ? ' – ' + esc(ex.to) : ''}</span>
                                    </div>
                                    ${ex.responsibilities?.length ? `
                                    <ul class="exp-resp">
                                        ${ex.responsibilities.map(r => `<li>${esc(r)}</li>`).join('')}
                                    </ul>` : ''}
                                </div>`).join('')}
                            </div>
                        </div>` : ''}

                        ${d.candidate?.education?.length ? `
                        <div class="find-section">
                            <div class="section-title">Education</div>
                            <div class="edu-list">
                                ${d.candidate.education.map(e => `
                                <div class="edu-item">
                                    <span class="edu-degree">${esc(e.degree ?? '')}</span>
                                    <span class="edu-inst">${esc(e.institution ?? '')}</span>
                                    ${e.year ? `<span class="edu-year cell-mono">${esc(e.year)}</span>` : ''}
                                </div>`).join('')}
                            </div>
                        </div>` : ''}

                        ${d.candidate?.projects?.length ? `
                        <div class="find-section">
                            <div class="section-title">Projects</div>
                            <div class="exp-list">
                                ${d.candidate.projects.map(p => `
                                <div class="exp-item">
                                    <div class="exp-header">
                                        <span class="exp-role cell-mono">${esc(p.name ?? '')}</span>
                                    </div>
                                    ${p.description?.length ? `
                                    <ul class="exp-resp">
                                        ${p.description.map(line => `<li>${esc(line)}</li>`).join('')}
                                    </ul>` : ''}
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
                                <div class="info-row"><span class="info-label">Department</span><span class="info-value">${esc(d.job?.department ?? '')}</span></div>
                                <div class="info-row"><span class="info-label">Location</span><span class="info-value">${esc(d.job?.location ?? '')}</span></div>
                                <div class="info-row"><span class="info-label">Pipeline Step</span><span class="info-value">${esc(d.job?.current_pipeline_step?.name ?? '')}</span></div>
                            </div>
                        </div>

                        ${d.job?.description?.length ? d.job.description.map(sec => `
                        <div class="find-section">
                            <div class="section-title">${esc(sec.title ?? '')}</div>
                            ${sec.description ? `<p class="summary-text">${esc(sec.description)}</p>` : ''}
                            ${sec.points?.length ? `
                            <ul class="job-points">
                                ${sec.points.map(pt => `<li>${esc(pt)}</li>`).join('')}
                            </ul>` : ''}
                        </div>`).join('') : ''}

                        ${d.round?.expected_skills?.length ? `
                        <div class="find-section">
                            <div class="section-title">Evaluation Areas</div>
                            <div class="eval-grid">
                                ${d.round.expected_skills.map(s => `
                                <div class="eval-item">
                                    <div class="eval-skill">${esc(s.skill ?? '')}</div>
                                    <div class="eval-desc">${esc(s.description ?? '')}</div>
                                </div>`).join('')}
                            </div>
                        </div>` : ''}

                    </div>
                </div>

                <!-- QUESTIONS -->
                <div id="tab-questions" class="find-panel">
                    <div class="find-scroll">

                        ${d.question_bank?.focus_areas?.length ? `
                        <div class="find-section">
                            <div class="section-title">Focus Areas</div>
                            <div class="skill-tags">
                                ${d.question_bank.focus_areas.map(a => `<span class="skill-tag">${esc(a)}</span>`).join('')}
                            </div>
                        </div>` : ''}

                        ${d.question_bank?.questions?.length ? `
                        <div class="find-section">
                            <div class="section-title">
                                Questions
                                <span class="section-badge">${d.question_bank.questions.length}</span>
                            </div>
                            <div class="qbank-list">
                                ${d.question_bank.questions.map((q, idx) => `
                                <div class="qbank-item">
                                    <div class="qbank-meta">
                                        <span class="qbank-idx">${idx + 1}</span>
                                        <span class="skill-tag">${esc(q.category ?? '')}</span>
                                        <span class="diff-badge diff-${(q.difficulty ?? '').toLowerCase()}">${esc(q.difficulty ?? '')}</span>
                                        <span class="qbank-time cell-mono">~${q.estimated_time_minutes ?? '?'}m</span>
                                        <span class="qbank-threshold cell-mono">pass ≥${Math.round((q.pass_threshold ?? 0) * 100)}%</span>
                                    </div>
                                    <div class="qbank-q">${esc(q.question ?? '')}</div>
                                    ${q.ideal_answer_keywords?.length ? `
                                    <div class="qbank-keywords">
                                        <span class="kw-label">keywords</span>
                                        ${q.ideal_answer_keywords.map(k => `<span class="kw-tag">${esc(k)}</span>`).join('')}
                                    </div>` : ''}
                                    ${q.followup_triggers?.length ? `
                                    <div class="qbank-followups">
                                        ${q.followup_triggers.map(f => `
                                        <div class="followup-row">
                                            <span class="followup-cond">${esc(f.condition ?? '')}</span>
                                            <span class="followup-q">↳ ${esc(f.follow_up ?? '')}</span>
                                        </div>`).join('')}
                                    </div>` : ''}
                                </div>`).join('')}
                            </div>
                        </div>` : ''}

                    </div>
                </div>

            </div>
        `
    } catch (err) {
        body.innerHTML = `<div class="table-error">Failed to load: ${esc(String(err?.message ?? err))}</div>`
    }
}

function switchTab(name, btn) {
    document.querySelectorAll('.find-tab').forEach(t => t.classList.remove('active'))
    document.querySelectorAll('.find-panel').forEach(p => p.classList.remove('active'))
    btn.classList.add('active')
    document.getElementById('tab-' + name)?.classList.add('active')
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
                    <button id="infer-btn" class="ghost-btn" onclick="inferNextQuestion()">✦ Ask</button>
                </div>

                <!-- Current active question card — updated live as the interview progresses -->
                <div id="current-question-card" class="current-q-card">
                    <div class="current-q-label">Current Question</div>
                    <div id="current-q-text" class="current-q-text">Waiting for session to start…</div>
                </div>

                <!-- NQI suggestion stream -->
                <div id="nqi-messages"></div>

                <div id="chat-bar">
                    <input id="chat-input" class="field-input" type="text"
                           placeholder="Guide next question… (or leave blank for auto)" />
                    <button id="send-btn" onclick="inferNextQuestion()">➤</button>
                </div>
            </div>

        </div>

        <div id="err-log"></div>
    `

    document.getElementById('chat-input').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') inferNextQuestion()
    })

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
    messages.scrollTop = messages.scrollHeight

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

    // Render the active session shell; timer is held until recording is live
    renderActiveSession(/* timerDeferred = */ true)

    const timerEl = document.getElementById('rec-timer')
    if (timerEl) timerEl.textContent = 'Loading…'

    try {
        // 1. Fetch interview from ATS, convert question bank, seed Redis
        const beginResult = await window.go.main.App.ATSBeginSession(_selectedInterview)
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
    const txArea = document.getElementById('tx-area')
    if (historyMode) {
        btn.textContent = '🔴 Live'; btn.classList.add('active')
        txArea.classList.add('history-mode'); rebuildHistoryView()
    } else {
        btn.textContent = '📜 History'; btn.classList.remove('active')
        txArea.classList.remove('history-mode'); updateLiveDisplay()
    }
}

function toggleRec() {
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
function inferNextQuestion() {
    const input = document.getElementById('chat-input')
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
function updateLiveDisplay() {
    const area = document.getElementById('tx-area')
    if (!area) return
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
    messages.scrollTop = messages.scrollHeight
}

// ── Device loader ──────────────────────────────────────────────────────────
function loadAudioDevices() {
    window.go.main.App.ListAudioDevices()
        .then(devices => {
            const micList = document.getElementById('mic-list')
            micList.innerHTML = ''
            if (devices.Mics?.length) {
                devices.Mics.forEach(mic => {
                    const label = document.createElement('label')
                    label.className = 'device-option'
                    const radio = document.createElement('input')
                    radio.type = 'radio'; radio.name = 'microphone'; radio.value = mic.Name
                    radio.addEventListener('change', () => onDeviceSelect('mic', mic.Name))
                    const span = document.createElement('span')
                    span.className = 'device-name'; span.textContent = mic.Name
                    label.appendChild(radio); label.appendChild(span)
                    micList.appendChild(label)
                })
            } else {
                micList.innerHTML = '<div class="no-devices">No microphones found</div>'
            }

            const speakerList = document.getElementById('speaker-list')
            speakerList.innerHTML = ''
            if (devices.Speakers?.length) {
                devices.Speakers.forEach(speaker => {
                    const label = document.createElement('label')
                    label.className = 'device-option'
                    const radio = document.createElement('input')
                    radio.type = 'radio'; radio.name = 'speaker'; radio.value = speaker.Name
                    radio.addEventListener('change', () => onDeviceSelect('speaker', speaker.Name))
                    const span = document.createElement('span')
                    span.className = 'device-name'; span.textContent = speaker.Name
                    label.appendChild(radio); label.appendChild(span)
                    speakerList.appendChild(label)
                })
            } else {
                speakerList.innerHTML = '<div class="no-devices">No speakers found</div>'
            }

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
