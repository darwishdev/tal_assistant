// ── Start session / device select page ──────────────────────────────────────

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
                <h4>🖥 Screen
                    <span class="device-section-hint">optional — leave blank to record full desktop</span>
                </h4>
                <div id="screen-list" class="device-list">
                    <div class="loading">Loading screens…</div>
                </div>
            </div>

        </div>

        <div id="rec-bar">
            <button id="rec-btn" onclick="startSessionAndRecord()" disabled>▶ Start Recording</button>
        </div>
    `
    // Reset selections so each visit requires a fresh pick
    selectedMic    = null
    selectedSpeaker = null
    selectedScreen  = null
    loadAudioDevices()
}

// ── Device loader ─────────────────────────────────────────────────────────────

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
                        defTag.style.background = '#e2e8f0'
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
                        defTag.style.background = '#e2e8f0'
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

            // "Full desktop" option — always present, selected by default
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
            document.getElementById('mic-list').innerHTML     = '<div class="error">Error loading devices</div>'
            document.getElementById('speaker-list').innerHTML = '<div class="error">Error loading devices</div>'
        })
}

// Called when the user selects a device radio button
function onDeviceSelect(type, device) {
    if (type === 'mic')     selectedMic     = device
    else if (type === 'speaker') selectedSpeaker = device
    else if (type === 'screen')  selectedScreen  = device
    document.getElementById('rec-btn').disabled = !(selectedMic && selectedSpeaker)
}

// ── Session start ─────────────────────────────────────────────────────────────
// 1. Fetch interview from ATS + seed Redis question bank (ATSBeginSession)
// 2. Open audio capture + STT stream (StartRecording)
// 3. Transition to active session view

async function startSessionAndRecord() {
    if (!selectedMic || !selectedSpeaker) {
        alert('Please select both microphone and speaker')
        return
    }
    if (!_selectedInterview) {
        alert('No interview selected — please go back and choose an interview')
        return
    }

    // Render session shell immediately; keep the timer paused until recording is live
    renderActiveSession(/* timerDeferred = */ true)

    const timerEl = document.getElementById('rec-timer')
    if (timerEl) timerEl.textContent = 'Loading…'

    try {
        // Step 1 — initialise session in Go (fetches ATS data, seeds Redis)
        const beginResult = await window.go.main.App.ATSBeginSession(_selectedInterview)
        if (beginResult !== 'ok') {
            showError('Session init failed: ' + beginResult)
            navigate('start_session')
            return
        }

        // Step 2 — start audio capture and STT stream
        const recResult = await window.go.main.App.StartRecording(
            selectedMic, selectedSpeaker, selectedScreen ?? ''
        )
        if (recResult !== 'ok') {
            showError('Recording failed: ' + recResult)
            navigate('start_session')
            return
        }

        // Step 3 — recording is live, start the visual MM:SS timer
        _startRecTimer()

    } catch (err) {
        showError('Unexpected error: ' + err)
        navigate('start_session')
    }
}
