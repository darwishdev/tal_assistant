// ── Login page ───────────────────────────────────────────────────────────────

function renderLogin() {
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
                <div id="login-err" class="login-err" style="display:none"></div>
                <button id="login-btn" class="login-btn" type="submit">Sign In</button>
            </form>
        </div>
    `
    // Auto-focus username field after the DOM is ready
    setTimeout(() => document.getElementById('login-usr')?.focus(), 50)
}

async function submitLogin(e) {
    e.preventDefault()
    const usr   = document.getElementById('login-usr').value.trim()
    const pwd   = document.getElementById('login-pwd').value
    const btn   = document.getElementById('login-btn')
    const errEl = document.getElementById('login-err')

    btn.disabled    = true
    btn.textContent = 'Signing in…'
    errEl.style.display = 'none'

    try {
        await window.go.main.App.ATSLogin(usr, pwd)
        navigate('interview_list')
    } catch (err) {
        errEl.textContent   = err?.message ?? String(err)
        errEl.style.display = 'block'
        btn.disabled        = false
        btn.textContent     = 'Sign In'
    }
}
