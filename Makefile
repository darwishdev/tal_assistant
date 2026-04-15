GEMINI_API_KEY=asd
GOOGLE_API_KEY=asd
GOOGLE_PROJECT_ID=gen-lang-client-0165069269

.PHONY: run dev build build-windows build-release clean g_auth

run:
	GDK_BACKEND=x11 wails dev web

dev:
	wails dev

build:
	wails build

build-windows:
	wails build -platform windows/amd64

build-release:
	wails build -clean -ldflags "-w -s"

build-windows-release:
	wails build -platform windows/amd64 -clean -ldflags "-w -s"

clean:
	rm -rf build/bin

g_auth:
	gcloud auth application-default login
