// ── Interview find / detail page ─────────────────────────────────────────────

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
        if (body) body.innerHTML = '<div class="table-error">No interview selected.</div>'
        return
    }

    try {
        const d = await window.go.main.App.ATSInterviewFind(_selectedInterview)

        // Update page title once data arrives
        const titleEl = document.getElementById('find-title')
        if (titleEl) titleEl.textContent = d.candidate?.name ?? 'Interview Detail'

        body.innerHTML = `

            <!-- ── Sticky hero bar ── -->
            <div class="find-hero">
                <div class="hero-left">
                    <div class="hero-name">${esc(d.candidate?.name ?? '')}</div>
                    <div class="hero-meta">
                        <span class="hero-meta-item">✉ ${esc(d.candidate?.email ?? '')}</span>
                        ${d.candidate?.phone
                            ? `<span class="hero-meta-item">📞 ${esc(d.candidate.phone)}</span>`
                            : ''}
                    </div>
                </div>
                <div class="hero-chips">
                    <span class="status-badge status-${esc((d.interview?.status ?? '').toLowerCase())}">
                        ${esc(d.interview?.status ?? '')}
                    </span>
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
                                <div class="info-row">
                                    <span class="info-label">Title</span>
                                    <span class="info-value">${esc(d.job?.title ?? '')}</span>
                                </div>
                                <div class="info-row">
                                    <span class="info-label">Department</span>
                                    <span class="info-value">${esc(d.job?.department ?? '')}</span>
                                </div>
                                <div class="info-row">
                                    <span class="info-label">Location</span>
                                    <span class="info-value">${esc(d.job?.location ?? '')}</span>
                                </div>
                                <div class="info-row">
                                    <span class="info-label">Pipeline Step</span>
                                    <span class="info-value">${esc(d.job?.current_pipeline_step?.name ?? '')}</span>
                                </div>
                            </div>
                        </div>

                        ${d.job?.description?.length
                            ? d.job.description.map(sec => `
                        <div class="find-section">
                            <div class="section-title">${esc(sec.title ?? '')}</div>
                            ${sec.description ? `<p class="summary-text">${esc(sec.description)}</p>` : ''}
                            ${sec.points?.length ? `
                            <ul class="job-points">
                                ${sec.points.map(pt => `<li>${esc(pt)}</li>`).join('')}
                            </ul>` : ''}
                        </div>`).join('')
                            : ''}

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
                                        <span class="diff-badge diff-${esc((q.difficulty ?? '').toLowerCase())}">${esc(q.difficulty ?? '')}</span>
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

// Switch visible tab panel
function switchTab(name, btn) {
    document.querySelectorAll('.find-tab').forEach(t => t.classList.remove('active'))
    document.querySelectorAll('.find-panel').forEach(p => p.classList.remove('active'))
    btn.classList.add('active')
    document.getElementById('tab-' + name)?.classList.add('active')
}
