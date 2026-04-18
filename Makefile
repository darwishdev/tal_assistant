GEMINI_API_KEY=asd
GOOGLE_API_KEY=asd
GOOGLE_PROJECT_ID=gen-lang-client-0165069269

.PHONY: run dev build build-windows build-release build-windows-release clean g_auth prepare-ffmpeg prepare-credentials prepare-config prepare-build

run:
	GDK_BACKEND=x11 wails dev web

dev:
	wails dev

build:
	wails build

build-windows:
	wails build -platform windows/amd64

# Prepare FFmpeg for bundling - copies system FFmpeg to build/bin if not already present
prepare-ffmpeg:
	@powershell -NoProfile -Command "\
		if (-not (Test-Path 'build/bin/ffmpeg.exe')) { \
			Write-Host 'Copying FFmpeg to build/bin...'; \
			New-Item -ItemType Directory -Force -Path 'build/bin' | Out-Null; \
			$$ffmpegPath = (Get-Command ffmpeg -ErrorAction SilentlyContinue).Source; \
			if ($$ffmpegPath) { \
				if ($$ffmpegPath -like '*shim*') { \
					$$realPath = $$ffmpegPath -replace 'shims', 'apps' -replace '.exe', '\\current\\bin\\ffmpeg.exe'; \
					if (-not (Test-Path $$realPath)) { \
						$$realPath = (Get-ChildItem (Split-Path $$ffmpegPath -Parent | Split-Path -Parent) -Recurse -Filter 'ffmpeg.exe' -ErrorAction SilentlyContinue | Where-Object { $$_.Length -gt 1MB } | Select-Object -First 1).FullName; \
					} \
					if ($$realPath) { $$ffmpegPath = $$realPath; } \
				} \
				Copy-Item $$ffmpegPath 'build/bin/ffmpeg.exe' -Force; \
				Write-Host '✓ FFmpeg copied successfully'; \
			} else { \
				Write-Host '✗ FFmpeg not found in PATH. Please install FFmpeg or manually copy ffmpeg.exe to build/bin/'; \
				exit 1; \
			} \
		} else { \
			Write-Host '✓ FFmpeg already present in build/bin'; \
		}"

# Prepare credentials for embedding - copies service account key to config folder
prepare-credentials:
	@powershell -NoProfile -Command "\
		if (-not (Test-Path 'config/application_default_credentials.json')) { \
			Write-Host 'Copying credentials to config for embedding...'; \
			if (Test-Path 'credentials/service-account-key.json') { \
				Copy-Item 'credentials/service-account-key.json' 'config/application_default_credentials.json' -Force; \
				Write-Host '✓ Credentials copied successfully'; \
			} else { \
				Write-Host '✗ credentials/service-account-key.json not found!'; \
				exit 1; \
			} \
		} else { \
			Write-Host '✓ Credentials already present in config'; \
		}"

# Prepare app.env for bundling - copies config/app.env to build/bin/config/
prepare-config:
	@powershell -NoProfile -Command "\
		New-Item -ItemType Directory -Force -Path 'build/bin/config' | Out-Null; \
		if (Test-Path 'config/app.env') { \
			Copy-Item 'config/app.env' 'build/bin/config/app.env' -Force; \
			Write-Host '✓ app.env staged to build/bin/config/'; \
		} else { \
			Write-Host '✗ config/app.env not found!'; \
			exit 1; \
		}"

# Prepare everything needed for production build
prepare-build: prepare-ffmpeg prepare-credentials prepare-config
	@echo "✓ Build preparation complete"

build-release: prepare-build
	wails build -clean -ldflags "-w -s"

build-windows: prepare-build
	wails build --platform windows/amd64 --nsis

build-windows-release: prepare-build
	wails build -platform windows/amd64 -clean -ldflags "-w -s"

clean:
	rm -rf build/bin

g_auth:
	gcloud auth application-default login
