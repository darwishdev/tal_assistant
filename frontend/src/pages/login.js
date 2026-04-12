// ── Login page ───────────────────────────────────────────────────────────────

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
    // Auto-focus username field after the DOM is ready
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
    const usr   = document.getElementById('login-usr').value.trim()
    const pwd   = document.getElementById('login-pwd').value
    const remember = document.getElementById('remember-me')?.checked ?? true
    const btn   = document.getElementById('login-btn')
    const errEl = document.getElementById('login-err')

    btn.disabled    = true
    btn.textContent = 'Signing in…'
    errEl.style.display = 'none'

    try {
        const loginResponse = await window.go.main.App.ATSLogin(usr, pwd)
        
        // Save session to localStorage if remember me is checked
        if (remember) {
            saveLoginSession(usr, pwd, loginResponse)
        }
        
        navigate('interview_list')
    } catch (err) {
        errEl.textContent   = err?.message ?? String(err)
        errEl.style.display = 'block'
        btn.disabled        = false
        btn.textContent     = 'Sign In'
    }
}
