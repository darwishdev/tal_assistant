// ── State ──────────────────────────────────────────────────────────────────
let recording = false
let srtLines = []
let sigLines = [`# Signals — ${new Date().toISOString()}`, '']
let selectedMic = null
let selectedSpeaker = null
let historyMode = false
let transcriptHistory = []
let currentPartial = { label: '', text: '' }

// How many recent transcript lines to attach automatically on every NQI call
const NQI_CONTEXT_LINES = 10

// ── Wails event listeners ──────────────────────────────────────────────────
window.runtime.EventsOn('status', (s) => {
    const btn = document.getElementById('rec-btn')
    const historyToggle = document.getElementById('history-toggle')
    const inferBtn = document.getElementById('infer-btn')
    const app = document.getElementById('app')
    const deviceSelector = document.getElementById('device-selector')

    if (s === 'recording') {
        app.classList.remove('init')
        deviceSelector.style.display = 'none'
        btn.textContent = '■ Stop'; btn.className = 'stop'
        historyToggle.style.display = 'inline-block'
        inferBtn.style.display = 'inline-block'
        recording = true
        transcriptHistory = []
        currentPartial = { label: '', text: '' }
    } else if (s === 'connecting') {
        app.classList.remove('init')
        btn.textContent = 'connecting...'
    } else {
        btn.textContent = '▶ Start'; btn.className = ''
        historyToggle.style.display = 'none'
        inferBtn.style.display = 'none'
        recording = false
        hidePartial()
        if (selectedMic && selectedSpeaker) {
            deviceSelector.style.display = 'none'
        }
    }
})

window.runtime.EventsOn('transcript', (d) => {
    console.log(d)

    if (d.isFinal) {
        transcriptHistory.push({ label: d.label, text: d.text })

        srtLines.push(`${srtLines.filter(l => l.match(/^\d+$/)).length + 1}`)
        srtLines.push(`${msToSRT(d.startMs)} --> ${msToSRT(d.endMs)}`)
        srtLines.push(`[${d.label}] ${d.text}`)
        srtLines.push('')

        currentPartial = { label: '', text: '' }

        if (historyMode) {
            addLineToHistory(d.label, d.text)
            hidePartial()
        } else {
            updateLiveDisplay()
        }
    } else {
        currentPartial = { label: d.label, text: d.text }

        if (historyMode) {
            showPartial(d.label, d.text)
        } else {
            updateLiveDisplay()
        }
    }
})

// window.runtime.EventsOn('signal', (d) => {
//     addSignalTag(d.signal, d.timestamp)
//     sigLines.push(`[${d.timestamp}] ${d.signal}`)
// })

window.runtime.EventsOn('error', (msg) => {
    showError(msg)
})

window.runtime.EventsOn('saved', (msg) => {
    showError('✓ ' + msg)
})

// // Auto NQI suggestion pushed from Go when the signal detector triggers it
// window.runtime.EventsOn('nqi_result', (d) => {
//     appendNQIMessage('agent', d.next_question || '', d.rationale || '')
// })

// ── Controls ───────────────────────────────────────────────────────────────
function toggleHistoryMode() {
    historyMode = !historyMode
    const btn = document.getElementById('history-toggle')
    const txArea = document.getElementById('tx-area')

    if (historyMode) {
        btn.textContent = '🔴 Live'
        btn.classList.add('active')
        txArea.classList.add('history-mode')
        rebuildHistoryView()
    } else {
        btn.textContent = '📜 History'
        btn.classList.remove('active')
        txArea.classList.remove('history-mode')
        updateLiveDisplay()
    }
}
function toggleRec() {
    if (!recording) {
        if (!selectedMic || !selectedSpeaker) {
            showError('Please select both microphone and speaker')
            return
        }
        window.go.main.App.StartRecording(selectedMic, selectedSpeaker, selectedScreen ?? '')
            .then(r => {
                if (r !== 'ok') showError(r)
            })
    } else {
        window.go.main.App.StopRecording()
    }
}

function onDeviceSelect(type, device) {
    if (type === 'mic') selectedMic = device
    else if (type === 'speaker') selectedSpeaker = device
    else if (type === 'screen') selectedScreen = device  // null = full desktop

    document.getElementById('rec-btn').disabled = !(selectedMic && selectedSpeaker)
}

function saveFiles() {
    const srt = srtLines.join('\n')
    const sigs = sigLines.join('\n')
    window.go.main.App.SaveFiles(srt, sigs)
        .then(r => showError(r === '' ? 'saved!' : r))
}

// ── Drag ───────────────────────────────────────────────────────────────────
function dragStart(e) {
    if (e.target.closest('.t-btns')) return
    e.preventDefault()
    try {
        if (window.runtime && window.runtime.WindowStartDrag) {
            window.runtime.WindowStartDrag()
        }
    } catch (err) {
        console.error('Drag error:', err)
    }
}

// ── NQI — infer next question ──────────────────────────────────────────────
// Triggered by: ✨ button, ➤ button, or Enter key in chat input.
//
// Behaviour:
//   - Always attaches the last NQI_CONTEXT_LINES transcript lines as context.
//   - prompt = whatever the user typed (may be empty string).
//     Go side decides: empty prompt → AUTO (use last Q/A pair);
//                      non-empty   → MANUAL (use prompt + transcript).
//   - Result (next_question + rationale) is shown in the err-log area.
//   - Button is disabled while the call is in-flight.
function inferNextQuestion() {
    const input = document.getElementById('chat-input')
    const inferBtn = document.getElementById('infer-btn')
    const sendBtn = document.getElementById('send-btn')
    const prompt = input.value.trim()
    // show user bubble only for manual prompts
    input.value = ""
    // Build transcript context automatically — user never has to think about it
    const recentTranscript = transcriptHistory
        .slice(-NQI_CONTEXT_LINES)
        .map(item => `${item.label}: ${item.text}`)
        .join('\n')

    // Disable controls while in-flight
    inferBtn.disabled = true
    sendBtn.disabled = true
    inferBtn.textContent = '⏳…'

    // Show the user's prompt as a bubble immediately (if non-empty)
    // if (prompt) appendNQIMessage('user', prompt, '')
    if (prompt) appendUserBubble(prompt)
    window.go.main.App.InferNextQuestion(prompt, recentTranscript)
        .then(result => {
            // Wails can return either a plain string or an already-decoded object
            // depending on how the Go binding is typed. Handle both.
            let parsed
            if (typeof result === 'string' && result !== '') {
                try { parsed = JSON.parse(result) } catch (_) { parsed = null }
            } else if (result && typeof result === 'object') {
                parsed = result
            }

            if (parsed && parsed.next_question) {
                // appendNQIMessage('agent', parsed.next_question, parsed.rationale || '')
                input.value = ''
            } else {
                showError('NQI: no suggestion returned.')
            }
        })
        .catch(err => {
            showError(`NQI error: ${err}`)
        })
        .finally(() => {
            inferBtn.disabled = false
            sendBtn.disabled = false
            inferBtn.textContent = '✨ Next Q'
        })
}

// ── UI helpers ─────────────────────────────────────────────────────────────
function updateLiveDisplay() {
    const area = document.getElementById('tx-area')
    const partial = document.getElementById('partial')

    area.querySelectorAll('.line:not(#partial)').forEach(l => l.remove())

    if (currentPartial.text) {
        document.getElementById('p-lbl').style.display = 'none'
        document.getElementById('p-tx').textContent = currentPartial.text
        partial.style.display = 'flex'
        partial.classList.add('live-mode-text')
        area.appendChild(partial)
    } else {
        partial.style.display = 'none'
        partial.classList.remove('live-mode-text')
    }

    area.scrollTop = area.scrollHeight
}

function rebuildHistoryView() {
    const area = document.getElementById('tx-area')
    const partial = document.getElementById('partial')

    area.querySelectorAll('.line:not(#partial)').forEach(l => l.remove())
    transcriptHistory.forEach(item => addLineToHistory(item.label, item.text))

    if (currentPartial.text) {
        document.getElementById('p-lbl').style.display = ''
        partial.classList.remove('live-mode-text')
        showPartial(currentPartial.label, currentPartial.text)
    } else {
        hidePartial()
    }
}

function addLineToHistory(label, text) {
    const area = document.getElementById('tx-area')
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
    document.getElementById('p-lbl').textContent = label
    document.getElementById('p-lbl').className = `lbl ${label.toLowerCase()}`
    document.getElementById('p-tx').textContent = text
    p.style.display = 'flex'
    document.getElementById('tx-area').appendChild(p)
    document.getElementById('tx-area').scrollTop = 99999
}

function hidePartial() {
    document.getElementById('partial').style.display = 'none'
}


// // ── NQI chat helpers ───────────────────────────────────────────────────────
// // Appends a chat bubble to #nqi-messages.
// // role: 'agent' (left, suggestion) | 'user' (right, prompt)
// function appendNQIMessage(role, text, rationale) {
//     const messages = document.getElementById('nqi-messages')
//     const wrap = document.createElement('div')
//     wrap.className = `nqi-msg nqi-${role}`

//     if (role === 'agent') {
//         wrap.innerHTML = `
//             <div class="nqi-bubble">
//                 <span class="nqi-label">✨ Suggested next question</span>
//                 <p class="nqi-text">${esc(text)}</p>
//                 ${rationale ? `<p class="nqi-rationale">💡 ${esc(rationale)}</p>` : ''}
//             </div>`
//     } else {
//         wrap.innerHTML = `<div class="nqi-bubble nqi-bubble-user">${esc(text)}</div>`
//     }

//     messages.appendChild(wrap)
//     messages.scrollTop = messages.scrollHeight
// }

function showError(msg) {
    const log = document.getElementById('err-log')
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

function msToSRT(ms) {
    const h = Math.floor(ms / 3600000)
    const m = Math.floor((ms % 3600000) / 60000)
    const s = Math.floor((ms % 60000) / 1000)
    const f = ms % 1000
    return `${pad(h)}:${pad(m)}:${pad(s)},${pad(f, 3)}`
}
function pad(n, l = 2) { return String(n).padStart(l, '0') }
function esc(t) { return t.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;') }

// ── Initialize Audio Devices ───────────────────────────────────────────────
function loadAudioDevices() {
    window.go.main.App.ListAudioDevices()
        .then(devices => {
            const micList = document.getElementById('mic-list')
            micList.innerHTML = ''
            if (devices.Mics && devices.Mics.length > 0) {
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
            if (devices.Speakers && devices.Speakers.length > 0) {
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

            // ── Screens ───────────────────────────────────────────────────
            const screenList = document.getElementById('screen-list')
            screenList.innerHTML = ''

            // full desktop option
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

            if (devices.Screens && devices.Screens.length > 0) {
                devices.Screens.forEach(screen => {
                    const label = document.createElement('label')
                    label.className = 'device-option screen-option'
                    label.innerHTML = `
            <input type="radio" name="screen" value="${screen.ID}">
            <div class="screen-preview">
                ${screen.Screenshot
                            ? `<img src="${screen.Screenshot}" alt="${screen.Name}" class="screen-thumb" />`
                            : `<div class="screen-thumb-placeholder">🖥</div>`
                        }
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

// ── Boot ───────────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
    const chatInput = document.getElementById('chat-input')
    if (chatInput) {
        chatInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') inferNextQuestion()
        })
    }
})

loadAudioDevices()
// Signal event — compact tag with tooltip
runtime.EventsOn("signal", (data) => {

    if (!data.type) return
    const bar = document.getElementById("sig-bar")
    console.log("signal detected", data)

    const chip = document.createElement("a")
    chip.className = `sig-chip ${data.type}`
    chip.textContent = data.type === "question" ? "Q" : "A"
    chip.title = `${data.timestamp}\n${data.text}`

    bar.appendChild(chip)
})

// NQI event — agent bubble (auto, from left)
runtime.EventsOn("nqi_result", (data) => {
    appendAgentBubble(data.next_question, data.rationale)
})

function appendAgentBubble(question, rationale) {
    const messages = document.getElementById("nqi-messages")
    const bubble = document.createElement("div")
    bubble.className = "nqi-bubble"
    bubble.innerHTML = `
        <span class="nqi-label">Suggested next question</span>
        <div class="nqi-q">${escapeHtml(question)}</div>
        <div class="nqi-rationale">${escapeHtml(rationale)}</div>
    `
    messages.appendChild(bubble)
    document.getElementById("nqi-chat").scrollTop = 99999

    const btn = document.getElementById("infer-btn")
    if (btn) { btn.classList.add("pulse"); setTimeout(() => btn.classList.remove("pulse"), 1200) }
}

function appendUserBubble(prompt) {
    const messages = document.getElementById("nqi-messages")
    const bubble = document.createElement("div")
    bubble.className = "nqi-user-bubble"
    bubble.innerHTML = `
        <span class="nqi-label">You</span>
        <div class="nqi-user-text">${escapeHtml(prompt)}</div>
    `
    messages.appendChild(bubble)
    document.getElementById("nqi-chat").scrollTop = 99999
}


function escapeHtml(s) {
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
}