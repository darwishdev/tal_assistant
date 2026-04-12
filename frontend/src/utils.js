// ── Utility functions ────────────────────────────────────────────────────────

// Escape HTML to prevent XSS when inserting user / API data into innerHTML
function esc(t) {
    return String(t ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
}

// Same alias for contexts where the full name is clearer
function escapeHtml(s) { return esc(s) }

// Convert milliseconds to SRT timestamp format  00:00:00,000
function msToSRT(ms) {
    const h = Math.floor(ms / 3600000)
    const m = Math.floor((ms % 3600000) / 60000)
    const s = Math.floor((ms % 60000) / 1000)
    const f = ms % 1000
    return `${pad(h)}:${pad(m)}:${pad(s)},${pad(f, 3)}`
}

function pad(n, l = 2) { return String(n).padStart(l, '0') }

// Append a timestamped line to the floating error log.
// Safe to call before the active-session view is rendered — silently no-ops.
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

// ── LocalStorage with TTL ───────────────────────────────────────────────────

// Save login session with TTL (default 24 hours)
function saveLoginSession(username, password, loginResponse, ttlHours = 24) {
    const expiresAt = Date.now() + (ttlHours * 60 * 60 * 1000)
    const session = {
        username,
        password,
        fullName: loginResponse.full_name,
        homePage: loginResponse.home_page,
        expiresAt
    }
    localStorage.setItem('ats_session', JSON.stringify(session))
}

// Get login session if valid (not expired)
function getLoginSession() {
    try {
        const stored = localStorage.getItem('ats_session')
        if (!stored) return null
        
        const session = JSON.parse(stored)
        
        // Check if expired
        if (Date.now() > session.expiresAt) {
            clearLoginSession()
            return null
        }
        
        return session
    } catch (err) {
        console.error('Failed to parse session:', err)
        clearLoginSession()
        return null
    }
}

// Clear login session
function clearLoginSession() {
    localStorage.removeItem('ats_session')
}

// Check if session exists and is valid
function hasValidSession() {
    return getLoginSession() !== null
}
