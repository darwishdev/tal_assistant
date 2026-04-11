// ── Shared application state ────────────────────────────────────────────────
// All mutable globals live here so every page module can read/write them
// without circular dependencies.

let _selectedInterview = null   // name (ATS docname) of the chosen interview
let recording          = false
let srtLines           = []
let sigLines           = [`# Signals — ${new Date().toISOString()}`, '']
let selectedMic        = null
let selectedSpeaker    = null
let selectedScreen     = null
let historyMode        = false
let transcriptHistory  = []
let currentPartial     = { label: '', text: '' }

const NQI_CONTEXT_LINES = 10
