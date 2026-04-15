# TAL Assistant - Interview Overlay Application

## About

TAL Assistant is an AI-powered interview overlay application built with Wails, Go, and modern web technologies. It provides real-time transcription, speaker diarization, automated question management, and intelligent interview assistance.

### Features

- **Real-time Transcription**: Google Cloud Speech-to-Text with speaker diarization
- **Signal Detection**: AI-powered extraction of questions and answers from conversations
- **Question Management**: Automated question bank with intelligent follow-ups
- **Answer Evaluation**: AI-powered judging agent with detailed feedback
- **Next Question Inference**: Smart suggestions for follow-up and change questions
- **Redis-based Orchestration**: Event-driven architecture for seamless agent coordination

## Prerequisites

- **Go**: 1.21 or higher
- **Node.js**: 16 or higher
- **Wails CLI**: Install with `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **Redis**: Running locally or accessible instance
- **Google Cloud Project**: With Speech-to-Text and Vertex AI APIs enabled

## Setup

### 1. Clone and Install Dependencies

```bash
git clone <repository-url>
cd tal_assistant
go mod tidy
cd frontend && npm install
```

### 2. Configure Google Cloud

#### Development Setup
For local development, use Application Default Credentials:

```bash
gcloud auth application-default login
```

#### Production Setup
See [PRODUCTION.md](PRODUCTION.md) for detailed instructions on creating and using service account credentials.

### 3. Configure Environment

Copy and edit the configuration file:

```bash
# Configuration is embedded in config/app.env
# Edit the values as needed
```

Key configuration variables:
- `GOOGLE_PROJECT_ID`: Your Google Cloud project ID
- `GOOGLE_API_KEY`: Gemini API key
- `GOOGLE_CREDENTIALS_PATH`: Path to service account key (production)
- `REDIS_HOST`: Redis server host (default: localhost)
- `ATS_BASE_URL`: ATS (Applicant Tracking System) API base URL

## Development

### Run in Development Mode

```bash
make dev
# or
wails dev
```

### Run with Custom Environment

```bash
make run
```

### Available Make Commands

- `make dev` - Run development mode
- `make build` - Build production executable
- `make build-windows` - Build for Windows (amd64)
- `make build-release` - Build optimized release version
- `make build-windows-release` - Build optimized Windows release
- `make clean` - Remove build artifacts
- `make g_auth` - Authenticate with Google Cloud

## Building for Production

For detailed production build instructions, including Google Cloud authentication setup, see [PRODUCTION.md](PRODUCTION.md).

### Quick Build

```bash
make build-windows-release
```

Output: `build/bin/interview-overlay.exe`

### Distribution

When distributing the application, include:
1. The executable (`interview-overlay.exe`)
2. Service account credentials file
3. Configuration file (optional, can use embedded defaults)

See [PRODUCTION.md](PRODUCTION.md) for complete distribution guidelines.

## Project Structure

```
tal_assistant/
├── app.go                  # Main application logic
├── main.go                 # Entry point
├── config/                 # Configuration management
│   ├── app.env            # Environment variables (embedded)
│   └── config.go          # Config loader
├── frontend/              # Wails frontend (HTML/JS/CSS)
│   └── src/
│       ├── main.js        # Main UI logic
│       ├── app.css        # Styles
│       └── pages/         # Page components
├── pkg/
│   ├── adk/               # AI agent implementations
│   │   ├── signalingagent/          # Signal extraction
│   │   ├── signalingagentmapper/    # Question mapping
│   │   ├── nextquestionindicator/   # Next question logic
│   │   ├── nextquestionextender/    # Follow-up generation
│   │   └── judgingagent/            # Answer evaluation
│   ├── redis/             # Redis orchestration
│   ├── stt/               # Speech-to-Text service
│   ├── ffmpeg/            # Audio/video capture
│   └── atsclient/         # ATS integration
└── build/                 # Build outputs
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./pkg/adk/tests -v

# Run with coverage
go test -cover ./...
```

## Architecture

The application uses an event-driven architecture with Redis as the message broker:

1. **Audio Capture** (FFmpeg) → captures microphone and speaker audio
2. **Speech-to-Text** (Google Cloud) → transcribes with speaker diarization
3. **Signaling Agent** → extracts Q&A signals from transcripts
4. **Signal Mapper** → maps signals to question bank
5. **Judging Agent** → evaluates answers and provides feedback
6. **Next Question Indicator** → determines if follow-up is needed
7. **Next Question Extender** → generates follow-up questions

All agents communicate through Redis pub/sub channels, with state cached in Redis.

## Troubleshooting

### Transcription not working
- Verify Google Cloud credentials are configured
- Check Speech-to-Text API is enabled
- Ensure audio devices are properly selected

### Build errors
- Run `make clean` and rebuild
- Verify Go version: `go version`
- Check Wails is installed: `wails version`

### Redis connection errors
- Ensure Redis is running: `redis-cli ping`
- Check Redis configuration in `config/app.env`

## Contributing

When contributing, please:
1. Run tests before submitting PRs
2. Follow Go code conventions
3. Update documentation for new features
4. Never commit credentials or API keys

## License

[Your License Here]

## Links

- [Wails Documentation](https://wails.io/docs/introduction)
- [Google Cloud Speech-to-Text](https://cloud.google.com/speech-to-text)
- [Vertex AI Gemini](https://cloud.google.com/vertex-ai/docs/generative-ai/model-reference/gemini)
