GEMINI_API_KEY=asd
GOOGLE_API_KEY=asd
GOOGLE_PROJECT_ID=gen-lang-client-0165069269


run:
	GDK_BACKEND=x11 wails dev web

g_auth:
	gcloud auth application-default login
