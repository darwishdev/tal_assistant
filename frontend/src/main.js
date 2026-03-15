        // ── State ──────────────────────────────────────────────────────────────────
        let recording = false
        let srtLines = []
        let sigLines = [`# Signals — ${new Date().toISOString()}`, '']

        const MIC_DEVICE = 'Microphone (Realtek(R) Audio)'
        const SPEAKER_DEVICE = 'Stereo Mix (Realtek(R) Audio)'

        // ── Wails event listeners ──────────────────────────────────────────────────
        window.runtime.EventsOn('status', (s) => {
            const btn = document.getElementById('rec-btn')
            const stat = document.getElementById('t-status')
            const app = document.getElementById('app')
            if (s === 'recording') {
                app.classList.remove('init')
                btn.textContent = '■ Stop'; btn.className = 'stop'
                stat.textContent = 'recording'; recording = true
            } else if (s === 'connecting') {
                app.classList.remove('init')
                btn.textContent = 'connecting...'; stat.textContent = 'connecting'
            } else {
                btn.textContent = '▶ Start'; btn.className = ''
                stat.textContent = 'idle'; recording = false
                hidePartial()
            }
        })

        window.runtime.EventsOn('transcript', (d) => {
            if (d.isFinal) {
                hidePartial()
                addLine(d.label, d.text)
                // Build SRT entry
                srtLines.push(`${srtLines.filter(l => l.match(/^\d+$/)).length + 1}`)
                srtLines.push(`${msToSRT(d.startMs)} --> ${msToSRT(d.endMs)}`)
                srtLines.push(`[${d.label}] ${d.text}`)
                srtLines.push('')
            } else {
                showPartial(d.label, d.text)
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
        function toggleRec() {
            if (!recording) {
                window.go.main.App.StartRecording(MIC_DEVICE, SPEAKER_DEVICE)
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
        function addLine(label, text) {
            const area = document.getElementById('tx-area')
            const div = document.createElement('div')
            div.className = 'line'
            div.innerHTML = `<span class="lbl ${label.toLowerCase()}">${label}</span><span class="tx">${esc(text)}</span>`
            area.appendChild(div)
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

        // ── Test ListAudioDevices ──────────────────────────────────────────────────
        window.go.main.App.ListAudioDevices()
            .then(devices => {
                console.log('Available audio devices:', devices)
                devices.forEach((device, index) => {
                    console.log(`  ${index + 1}. ${device}`)
                })
            })
            .catch(err => {
                console.error('Error listing audio devices:', err)
            })