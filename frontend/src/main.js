        // ── State ──────────────────────────────────────────────────────────────────
        let recording = false
        let srtLines = []
        let sigLines = [`# Signals — ${new Date().toISOString()}`, '']
        let selectedMic = null
        let selectedSpeaker = null
        let historyMode = false
        let transcriptHistory = []
        let currentPartial = { label: '', text: '' }

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
            console.log(d);
            
            if (d.isFinal) {
                // Add to history
                transcriptHistory.push({ label: d.label, text: d.text })
                
                // Build SRT entry
                srtLines.push(`${srtLines.filter(l => l.match(/^\d+$/)).length + 1}`)
                srtLines.push(`${msToSRT(d.startMs)} --> ${msToSRT(d.endMs)}`)
                srtLines.push(`[${d.label}] ${d.text}`)
                srtLines.push('')
                
                // Reset current partial
                currentPartial = { label: '', text: '' }
                
                // Update display based on mode
                if (historyMode) {
                    addLineToHistory(d.label, d.text)
                    hidePartial()
                } else {
                    updateLiveDisplay()
                }
            } else {
                // Update current partial
                // if(currentPartial){
                //     currentPartial.text = currentPartial.text + ' ' + d.text
                // }
                // else{
                    currentPartial = { label: d.label, text: d.text }
                // }
                
                if (historyMode) {
                    showPartial(d.label, d.text)
                } else {
                    updateLiveDisplay()
                }
            }
        })

        window.runtime.EventsOn('signal', (d) => {
            addSignalTag(d.signal, d.timestamp)
            sigLines.push(`[${d.timestamp}] ${d.signal}`)
        })

        window.runtime.EventsOn('error', (msg) => {
            showError(msg)
        })

        window.runtime.EventsOn('saved', (msg) => {
            showError('✓ ' + msg)  // reuse error bar for notifications
        })

        // ── Controls ───────────────────────────────────────────────────────────────
        function toggleHistoryMode() {
            historyMode = !historyMode
            const btn = document.getElementById('history-toggle')
            const txArea = document.getElementById('tx-area')
            
            if (historyMode) {
                btn.textContent = '🔴 Live'
                btn.classList.add('active')
                txArea.classList.add('history-mode')
                // Rebuild full history
                rebuildHistoryView()
            } else {
                btn.textContent = '📜 History'
                btn.classList.remove('active')
                txArea.classList.remove('history-mode')
                // Switch to live view
                updateLiveDisplay()
            }
        }

        function toggleRec() {
            if (!recording) {
                if (!selectedMic || !selectedSpeaker) {
                    showError('Please select both microphone and speaker')
                    return
                }
                window.go.main.App.StartRecording(selectedMic, selectedSpeaker)
                    .then(r => { 
                        if (r !== 'ok') {
                            showError(r)
                            console.log(r);
                        }
                        else{
                            console.log(r);
                        }
                      })
            } else {
                window.go.main.App.StopRecording()
            }
        }

        function onDeviceSelect(type, device) {
            if (type === 'mic') {
                selectedMic = device
            } else if (type === 'speaker') {
                selectedSpeaker = device
            }
            
            // Enable start button if both devices are selected
            const btn = document.getElementById('rec-btn')
            if (selectedMic && selectedSpeaker) {
                btn.disabled = false
            } else {
                btn.disabled = true
            }
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

        // ── UI helpers ─────────────────────────────────────────────────────────────
        function updateLiveDisplay() {
            const area = document.getElementById('tx-area')
            const partial = document.getElementById('partial')
            
            // Clear all lines except partial
            const lines = area.querySelectorAll('.line:not(#partial)')
            lines.forEach(line => line.remove())
            
            // In live mode: ONLY show current partial, no history, no speaker label
            if (currentPartial.text) {
                // Hide speaker label in live mode
                const lblElement = document.getElementById('p-lbl')
                lblElement.style.display = 'none'
                
                // Show text without speaker label
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
            
            // Clear all lines except partial
            const lines = area.querySelectorAll('.line:not(#partial)')
            lines.forEach(line => line.remove())
            
            // Add all history items
            transcriptHistory.forEach(item => {
                addLineToHistory(item.label, item.text)
            })
            
            // Show partial if exists (with speaker label in history mode)
            if (currentPartial.text) {
                // Re-enable speaker label for history mode
                const lblElement = document.getElementById('p-lbl')
                lblElement.style.display = ''
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
            const partial = document.getElementById('partial')
            area.insertBefore(div, partial)
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

        function addSignalTag(signal, ts) {
            const strip = document.getElementById('sig-bar')
            const noSig = document.getElementById('no-sig')
            if (noSig) noSig.remove()

            const tokens = signal.match(/\[[A-Z_]+\]/g) || [signal]
            for (const t of tokens) {
                const span = document.createElement('span')
                span.className = 'tag'; span.title = ts; span.textContent = t
                strip.appendChild(span)
            }
            // Keep max 6 tags
            const tags = strip.querySelectorAll('.tag')
            if (tags.length > 6) for (let i = 0; i < tags.length - 6; i++) tags[i].remove()
        }

        function showError(msg) {
            const log = document.getElementById('err-log')
            log.style.display = 'block'
            const line = document.createElement('div')
            line.className = 'err-line'
            const ts = new Date().toLocaleTimeString('en-US', { hour12: false })
            line.textContent = `[${ts}] ${msg}`
            log.appendChild(line)
            log.scrollTop = log.scrollHeight
            // Keep max 20 error lines
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
                    console.log('Available audio devices:', devices)
                    
                    // Populate microphones
                    const micList = document.getElementById('mic-list')
                    micList.innerHTML = ''
                    if (devices.mics && devices.mics.length > 0) {
                        devices.mics.forEach((mic, index) => {
                            const label = document.createElement('label')
                            label.className = 'device-option'
                            
                            const radio = document.createElement('input')
                            radio.type = 'radio'
                            radio.name = 'microphone'
                            radio.value = mic
                            radio.addEventListener('change', () => onDeviceSelect('mic', mic))
                            
                            const span = document.createElement('span')
                            span.className = 'device-name'
                            span.textContent = mic
                            
                            label.appendChild(radio)
                            label.appendChild(span)
                            micList.appendChild(label)
                        })
                    } else {
                        micList.innerHTML = '<div class="no-devices">No microphones found</div>'
                    }
                    
                    // Populate speakers
                    const speakerList = document.getElementById('speaker-list')
                    speakerList.innerHTML = ''
                    if (devices.speakers && devices.speakers.length > 0) {
                        devices.speakers.forEach((speaker, index) => {
                            const label = document.createElement('label')
                            label.className = 'device-option'
                            
                            const radio = document.createElement('input')
                            radio.type = 'radio'
                            radio.name = 'speaker'
                            radio.value = speaker
                            radio.addEventListener('change', () => onDeviceSelect('speaker', speaker))
                            
                            const span = document.createElement('span')
                            span.className = 'device-name'
                            span.textContent = speaker
                            
                            label.appendChild(radio)
                            label.appendChild(span)
                            speakerList.appendChild(label)
                        })
                    } else {
                        speakerList.innerHTML = '<div class="no-devices">No speakers found</div>'
                    }
                })
                .catch(err => {
                    console.error('Error listing audio devices:', err)
                    document.getElementById('mic-list').innerHTML = '<div class="error">Error loading devices</div>'
                    document.getElementById('speaker-list').innerHTML = '<div class="error">Error loading devices</div>'
                })
        }

        // ── Chat Functions ─────────────────────────────────────────────────────────
        function sendMessage() {
            const input = document.getElementById('chat-input')
            const message = input.value.trim()
            
            if (!message) return
            
            // TODO: Implement message sending logic
            // For now, just log and clear the input
            console.log('Sending message:', message)
            showError(`Message sent: ${message}`)
            
            input.value = ''
        }

        function inferNextQuestion() {
            // TODO: Implement AI inference logic
            console.log('Inferring next question...')
            showError('Inferring next question... (feature coming soon)')
        }

        // ── Chat Input Enter Key Handler ───────────────────────────────────────────
        document.addEventListener('DOMContentLoaded', () => {
            const chatInput = document.getElementById('chat-input')
            if (chatInput) {
                chatInput.addEventListener('keypress', (e) => {
                    if (e.key === 'Enter') {
                        sendMessage()
                    }
                })
            }
        })

        // Load devices on startup
        loadAudioDevices()