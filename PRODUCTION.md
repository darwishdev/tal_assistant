# Production Build Guide

This guide explains how to build and configure the application for production deployment, including authentication setup for Google Cloud services.

## Google Cloud Authentication for Production

The application uses Google Cloud Speech-to-Text API, which requires authentication. There are multiple authentication methods available:

### Development (Current Setup)
- Uses **Application Default Credentials (ADC)**
- Requires running `gcloud auth application-default login`
- Works on your development machine but NOT in production builds

### Production Options

#### ⚠️ Important: Service Account Key Restrictions
Many organizations have policies that **disable service account key creation** (`iam.disableServiceAccountKeyCreation`) for security reasons. If you encounter this restriction, use one of the alternative methods below.

#### Option 1: Use Your ADC Credentials (Simplest - Recommended for Small Deployments)
#### Option 1: Embedded ADC Credentials (Simplest - Recommended)
- **Single .exe file** - credentials compiled into the binary
- **Works immediately** - no separate files needed
- Zero setup for end users
- **Best for**: Internal tools, small user base, controlled distribution
- No gcloud required on user devices

#### Option 2: External ADC Credentials File
- Credentials in a separate file alongside the exe
- Easier to update without rebuilding
- Distribute exe + credentials folder
- **Best for**: When you need to update credentials frequently

#### Option 3: OAuth 2.0 User Consent (Best for Multi-User Apps)
- Users authenticate with their own Google account
- Requires user interaction on first launch
- No shared credentials
- Users must have Google Cloud access

#### Option 4: Workload Identity Federation
- Modern keyless authentication
- More complex setup but highly secure
- Best for enterprise deployments

#### Option 5: Service Account Keys (Legacy - Requires Admin Override)
- Traditional method using JSON key files
- **Blocked by default in many organizations**
- Requires Organization Policy Administrator to disable security constraint
- Only use if other options are not viable

## Authentication Setup

Choose the authentication method that best fits your deployment scenario and security requirements.

### Quick Decision Guide

| Scenario | Recommended Method |
|----------|-------------------|
| Single .exe distribution, no folders | **Option 1: Embedded ADC** ✅ |
| Need to update credentials easily | **Option 2: External ADC File** |
| Each user has their own Google account | **Option 3: OAuth 2.0** |
| Enterprise deployment with identity provider | **Option 4: Workload Identity Federation** |
| Organization blocks key creation | **Option 1, 2, or 3** (avoid Option 5) |

---

## Option 1: Use Embedded ADC Credentials (Simplest - Recommended)

This is the **easiest approach** for distributing your app:
- Credentials are **compiled directly into the executable**
- Users receive **only a single .exe file** - no folders or config files needed
- Works immediately without user authentication
- **Best for**: Internal tools, controlled distribution, small user base

### How It Works
- You authenticate once with `gcloud auth application-default login`
- Copy the credentials file to `config/application_default_credentials.json`
- Build the app - credentials are embedded via Go's `//go:embed` directive
- Distribute **only the .exe file**
- All users share your credentials (tied to your Google account)

### Setup Process

**Step 1: Authenticate and Copy Credentials**

```powershell
# Authenticate (if not already done)
gcloud auth application-default login

# Copy ADC file to config folder for embedding
Copy-Item "$env:APPDATA\gcloud\application_default_credentials.json" "config\application_default_credentials.json"
```

**Step 2: Build Your Application**

```powershell
# Build - credentials are automatically embedded
make build-windows-release
```

**Step 3: Distribute**

```powershell
# Just distribute the single executable!
# No credentials folder needed - everything is embedded
build/bin/interview-overlay.exe
```

### How It's Implemented

The credentials are embedded at compile time using Go's embed directive in `config/config.go`:

```go
//go:embed application_default_credentials.json
var embeddedCredentials []byte
```

At runtime, the app tries credentials in this priority order:
1. **External file** (if `GOOGLE_CREDENTIALS_PATH` is set and file exists)
2. **Embedded credentials** (compiled into the binary) ✅
3. **Application Default Credentials** (fallback for development)

### Security Considerations

**Keep credentials.json out of git:**
- ✅ Already in `.gitignore`: `config/application_default_credentials.json`
- The credentials are only in your compiled binary, not in source control

**Distribution:**
- Only distribute to trusted users within your organization
- Credentials give access under YOUR Google account
- All API usage appears under your account in Cloud Console

**Credential Owner:**
- Credentials tied to YOUR Google account
- If you leave the organization or lose access, all distributed apps stop working
- Consider using a dedicated "bot" Google account for production

**Expiration:**
- Refresh tokens may expire after ~6 months of inactivity
- You'll need to rebuild and redistribute if credentials expire

### Advantages
- ✅ **Single file distribution** - just the .exe, no folders
- ✅ **Zero setup for end users** - double-click and run
- ✅ **No gcloud required** on user machines
- ✅ **Works immediately** - no authentication prompts
- ✅ **Bypasses service account key restriction**
- ✅ **Auto-refreshing tokens** embedded in the credentials

### Disadvantages
- ⚠️ All users share your credentials
- ⚠️ No per-user audit trail
- ⚠️ Credentials embedded in the binary (can be extracted)
- ⚠️ If your account is compromised/disabled, all apps fail
- ⚠️ Need to rebuild and redistribute to update credentials

---

## Option 2: Use External ADC File (Alternative)

This is the **easiest approach** for distributing your app when:
- Users don't have gcloud installed
- App should work immediately without user authentication
- You're distributing to a small, trusted group
- Service account key creation is blocked

### How It Works
- You authenticate once with `gcloud auth application-default login`
- Copy the generated credentials file
- Bundle it with your application
- All users share your credentials (tied to your Google account)

### Step 1: Authenticate and Locate ADC File

```powershell
# Authenticate (if not already done)
gcloud auth application-default login

# The ADC file is created at:
# C:\Users\Kareem Dev\AppData\Roaming\gcloud\application_default_credentials.json
```

### Step 2: Copy ADC to Your Project

```powershell
# Create credentials folder
New-Item -ItemType Directory -Force -Path credentials

# Copy the ADC file
Copy-Item "$env:APPDATA\gcloud\application_default_credentials.json" "credentials\application_default_credentials.json"
```

### Step 3: Configure Your App

Update `config/app.env`:
```env
GOOGLE_CREDENTIALS_PATH=./credentials/application_default_credentials.json
GOOGLE_PROJECT_ID=gen-lang-client-0165069269
```

### Step 4: Add to .gitignore

Ensure your `.gitignore` includes:
```
credentials/
*.json
!wails.json
!package.json
```

### Step 5: Build and Distribute

```powershell
# Build the app
make build-windows-release

# Your distribution should include:
# - build/bin/interview-overlay.exe
# - credentials/application_default_credentials.json  
# - config/app.env (if not embedded)
```

### Distribution Structure
```
interview-overlay/
├── interview-overlay.exe
├── credentials/
│   └── application_default_credentials.json
└── config/
    └── app.env (optional)
```

### Testing Without Gcloud

To verify it works on user devices without gcloud:

1. **On a test machine without gcloud** (or temporarily rename gcloud folder):
```powershell
# Temporarily disable gcloud
Rename-Item "$env:APPDATA\gcloud" "$env:APPDATA\gcloud.backup"

# Run your app
.\interview-overlay.exe

# Should work without any authentication prompts!

# Restore gcloud
Rename-Item "$env:APPDATA\gcloud.backup" "$env:APPDATA\gcloud"
```

### Important Considerations

**Credential Owner**: 
- Credentials are tied to YOUR Google account
- If you leave the organization or lose access, all distributed apps stop working
- Consider using a dedicated service account user instead (e.g., a "bot" user account)

**Security**:
- Don't distribute to untrusted users (they get full access as you)
- Credentials include refresh token - can access your Cloud resources
- Keep distribution controlled and within your organization

**Expiration**:
- Refresh tokens may expire after ~6 months of inactivity
- You'll need to redistribute updated credentials if this happens
- Test periodically to ensure credentials haven't expired

**Best For**:
- Internal tools within your team
- Controlled distribution (5-50 users)
- Short-to-medium term deployments
- Situations where service account keys are blocked

### Advantages
- ✅ **Zero setup for end users** - just run the exe
- ✅ **No gcloud required** on user machines
- ✅ **Works immediately** - no authentication prompts
- ✅ **Bypasses service account key restriction**
- ✅ **No code changes needed**
- ✅ **Auto-refreshing tokens** included in the file

### Disadvantages
- ⚠️ All users share your credentials
- ⚠️ No per-user audit trail
- ⚠️ If your account is compromised/disabled, all apps fail
- ⚠️ Not suitable for wide public distribution

---

## Option 2: OAuth 2.0 User Consent (Best for Multi-User)

---

## Option 2: Use External ADC File (Alternative)

If you prefer to keep credentials as a separate file (useful for easier updates without rebuilding):

### How It Works
- You authenticate once with `gcloud auth application-default login`
- Copy the generated credentials file
- Bundle it with your application in a `credentials/` folder
- Users share your credentials (tied to your Google account)
- Easier to update credentials without rebuilding the app

### Step 1: Authenticate and Locate ADC File

```powershell
# Authenticate (if not already done)
gcloud auth application-default login

# The ADC file is created at:
# C:\Users\Kareem Dev\AppData\Roaming\gcloud\application_default_credentials.json
```

### Step 2: Copy ADC to Your Project

```powershell
# Create credentials folder
New-Item -ItemType Directory -Force -Path credentials

# Copy the ADC file
Copy-Item "$env:APPDATA\gcloud\application_default_credentials.json" "credentials\application_default_credentials.json"
```

### Step 3: Configure Your App

Update `config/app.env`:
```env
GOOGLE_CREDENTIALS_PATH=./credentials/application_default_credentials.json
GOOGLE_PROJECT_ID=gen-lang-client-0165069269
```

### Step 4: Build and Distribute

```powershell
# Build the app
make build-windows-release

# Manually copy credentials to build folder
Copy-Item -Recurse -Force "credentials" "build\bin\credentials"
```

### Distribution Structure
```
interview-overlay/
├── interview-overlay.exe
└── credentials/
    └── application_default_credentials.json
```

### Advantages vs Embedded
- ✅ Update credentials without rebuilding the app
- ✅ Easier to manage multiple credential sets
- ✅ Credentials not in the binary (slightly more secure)

### Disadvantages vs Embedded
- ⚠️ Must distribute multiple files/folders
- ⚠️ Users might separate the exe from credentials folder
- ⚠️ More complex distribution

---

## Option 3: OAuth 2.0 User Consent (Best for Multi-User)

### How It Works
- Application requests user consent on first launch
- User logs in with their Google account
- Credentials are cached locally for future use
- No service account keys needed

### Prerequisites
- Users need Google accounts with Cloud Speech API access
- OAuth 2.0 Client ID configured in Google Cloud Console

### Step 1: Create OAuth 2.0 Client

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Select project: `gen-lang-client-0165069269`
3. Navigate to **APIs & Services** → **Credentials**
4. Click **+ CREATE CREDENTIALS** → **OAuth client ID**
5. Configure:
   - **Application type**: Desktop app
   - **Name**: `TAL Assistant Desktop`
6. Click **CREATE**
7. Download the JSON file (e.g., `client_secret_xxx.json`)

### Step 2: Update Application Code

You'll need to implement OAuth 2.0 flow in your Go code. Here's the approach:

```go
// Add to config/config.go
type Config struct {
    // ... existing fields ...
    OAuthClientID     string `mapstructure:"OAUTH_CLIENT_ID"`
    OAuthClientSecret string `mapstructure:"OAUTH_CLIENT_SECRET"`
}

// In pkg/stt/sttservice.go, update authentication
import (
    "context"
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
)

func (s *STTService) initializeWithOAuth(ctx context.Context) error {
    config := &oauth2.Config{
        ClientID:     s.config.OAuthClientID,
        ClientSecret: s.config.OAuthClientSecret,
        Scopes:       []string{"https://www.googleapis.com/auth/cloud-platform"},
        Endpoint:     google.Endpoint,
    }
    
    // Token will be cached to avoid repeated logins
    token, err := s.getTokenFromCache()
    if err != nil {
        // Prompt user for consent
        token, err = s.getUserConsent(config)
    }
    
    // Use token for API calls
    client := config.Client(ctx, token)
    // Initialize Speech client with authenticated client
}
```

### Step 3: Configure Environment

Update `config/app.env`:
```env
OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
OAUTH_CLIENT_SECRET=your-client-secret
AUTH_METHOD=oauth
```

### Step 4: Test

Run the app - it should prompt for Google login on first use.

---

## Option 4: Workload Identity Federation

Best for enterprise deployments where the app runs in a controlled environment.

### Step 1: Set Up Workload Identity Pool

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Navigate to **IAM & Admin** → **Workload Identity Federation**
3. Click **CREATE POOL**
4. Configure:
   - **Name**: `tal-assistant-pool`
   - **Provider**: Choose based on your environment (OIDC, SAML, AWS, Azure)

### Step 2: Create Provider

Follow the wizard to create a provider based on your identity system.

### Step 3: Configure Application

The application will use workload identity configuration instead of keys:

```env
GOOGLE_PROJECT_ID=gen-lang-client-0165069269
GOOGLE_WORKLOAD_IDENTITY_PROVIDER=projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/POOL_ID/providers/PROVIDER_ID
```

This method requires more infrastructure setup but eliminates key management entirely.

---

## Option 5: Service Account Keys (Requires Admin Override)

**⚠️ Only use this if your organization approves and an admin can override the policy.**

### Understanding the Restriction

Your organization has enabled `iam.disableServiceAccountKeyCreation` policy to prevent security incidents. Service account keys are permanent credentials that, if leaked, can be exploited.

### Requesting Override

If you must use service account keys:

1. Contact your **Organization Policy Administrator**
2. Explain your use case for the desktop application
3. Request they disable the constraint for your specific project or service account
4. Provide tracking number: `c8661695267057193`

### If Override is Granted

Follow these steps to create and configure a service account key:

### Step 1: Create a Service Account

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Select your project: `gen-lang-client-0165069269`
3. Navigate to **IAM & Admin** → **Service Accounts**
4. Click **+ CREATE SERVICE ACCOUNT**
5. Fill in:
   - **Service account name**: `tal-assistant-production`
   - **Service account ID**: `tal-assistant-production`
   - **Description**: `Service account for TAL Assistant production app`
6. Click **CREATE AND CONTINUE**

### Step 2: Grant Permissions

Grant the following roles to the service account:
- **Cloud Speech Client** (for Speech-to-Text API)
- **Vertex AI User** (for Gemini API if needed)

Click **CONTINUE** then **DONE**

### Step 3: Create and Download Key

1. In the Service Accounts list, click on the newly created account
2. Go to the **KEYS** tab
3. Click **ADD KEY** → **Create new key**
4. Select **JSON** format
5. Click **CREATE**
6. The key file will be downloaded automatically (e.g., `tal-assistant-production-xxxxx.json`)

**⚠️ IMPORTANT**: Keep this file secure! It provides access to your Google Cloud resources.

### Step 4: Configure the Application

#### Option A: Store credentials file with the app

1. Create a `credentials` folder in your project:
   ```powershell
   mkdir credentials
   ```

2. Copy the downloaded service account key:
   ```powershell
   copy "C:\Users\YourName\Downloads\tal-assistant-production-xxxxx.json" "credentials\service-account-key.json"
   ```

3. Update your `config/app.env`:
   ```env
   GOOGLE_CREDENTIALS_PATH=./credentials/service-account-key.json
   ```

#### Option B: Use absolute path

Update your `config/app.env` with the full path:
```env
GOOGLE_CREDENTIALS_PATH=C:/path/to/your/service-account-key.json
```

### Step 5: Add to .gitignore

Ensure your `.gitignore` includes:
```
credentials/
*.json
!wails.json
!package.json
```

## Building for Production

### Standard Build
```powershell
make build
```
- Output: `build/bin/interview-overlay.exe`

### Optimized Release Build (Recommended)
```powershell
make build-windows-release
```
- Smaller file size (strips debug symbols)
- Better performance
- Output: `build/bin/interview-overlay.exe`

### Cross-Platform Build
```powershell
make build-windows
```

## Distributing the Application

### Using Embedded Credentials (Option 1 - Recommended)

When using embedded credentials, distribution is simple:

**Just distribute the single executable!**

```
interview-overlay.exe
```

Users can:
- ✅ Double-click to run - no setup needed
- ✅ Place it anywhere on their system
- ✅ No gcloud, no folders, no configuration required

### Using External ADC Credentials (Option 2)

When distributing with external credentials file:

1. **The executable**: `build/bin/interview-overlay.exe`
2. **ADC credentials**: `credentials/application_default_credentials.json`
3. **Config file**: `config/app.env` (optional override)

### Distribution Structure
```
interview-overlay/
├── interview-overlay.exe
└── credentials/
    └── application_default_credentials.json
```

**Users just run `interview-overlay.exe` - no gcloud, no authentication required!**

### Using Service Account Keys (Option 5 - If You Have Admin Override)

When distributing with service account keys:

1. **The executable**: `build/bin/interview-overlay.exe`
2. **Service account key**: `credentials/service-account-key.json`
3. **Config file**: `config/app.env` (optional override)

### Distribution Structure
```
interview-overlay/
├── interview-overlay.exe
└── credentials/
    └── service-account-key.json
```

### For End Users

Users can place a custom `app.env` file in:
- `./config/app.env` (next to the executable)
- `~/.config/tal_assistant/app.env` (user's home directory)

## Environment Variables

You can also set configuration via environment variables instead of the config file:

```powershell
$env:GOOGLE_CREDENTIALS_PATH="./credentials/service-account-key.json"
$env:GOOGLE_PROJECT_ID="gen-lang-client-0165069269"
$env:GOOGLE_API_KEY="your-api-key-here"
```

## Testing Production Authentication

### Testing ADC Credentials

To verify your ADC setup works:

1. **Copy the ADC file to your project**:
   ```powershell
   Copy-Item "$env:APPDATA\gcloud\application_default_credentials.json" "credentials\application_default_credentials.json"
   ```

2. **Update `config/app.env`**:
   ```env
   GOOGLE_CREDENTIALS_PATH=./credentials/application_default_credentials.json
   ```

3. **Temporarily disable your gcloud auth** (optional):
   ```powershell
   gcloud auth application-default revoke
   ```

4. **Run the app**:
   ```powershell
   wails dev
   ```

5. **Test transcription functionality**

If it works after revoking gcloud auth, your bundled credentials are working correctly!

### Testing Service Account Credentials

To verify your service account setup works:

1. Remove your personal credentials (optional):
   ```powershell
   gcloud auth application-default revoke
   ```

2. Set the credentials path in `config/app.env`

3. Run the app:
   ```powershell
   wails dev
   ```

4. Test transcription functionality

If it works without `gcloud auth`, your service account is configured correctly!

## Security Best Practices

### For ADC Credentials

1. **Control distribution carefully**
   - Only distribute to trusted users within your organization
   - Users get access with your credentials
   - Track who has copies of the app

2. **Monitor usage**
   - Check Cloud Console audit logs regularly
   - Watch for unexpected API usage
   - All actions appear under your account

3. **Limit permissions**
   - Ensure your account has only necessary roles
   - Don't use admin accounts for ADC distribution
   - Consider creating a dedicated "bot" Google account

4. **Never commit to git**
   - Add to `.gitignore`
   - Keep credentials out of version control
   - Don't share publicly

5. **Plan for credential refresh**
   - Test periodically (refresh tokens can expire)
   - Have a process to redistribute updated credentials
   - Document credential update procedures

### For Service Account Keys

1. **Never commit service account keys to git**
   - Add to `.gitignore`
   - Use secret management in production

2. **Rotate keys periodically**
   - Create new keys every 90 days
   - Delete old keys from Google Cloud Console

3. **Limit service account permissions**
   - Only grant necessary roles
   - Use separate service accounts for dev/prod

4. **Protect the key file**
   - Set restrictive file permissions
   - Encrypt in production environments
   - Consider using secret management services

## Troubleshooting

### "Could not load credentials"
- Verify the `GOOGLE_CREDENTIALS_PATH` points to the correct file
- Check the JSON file is valid and not corrupted
- Ensure the service account has necessary permissions

### "Permission denied" errors
- Check service account has correct roles (Cloud Speech Client)
- Verify the service account is in the correct project

### Build fails
- Run `make clean` first
- Ensure all dependencies are installed: `go mod tidy`
- Check your Go version: `go version` (should be 1.21+)

## Alternative: Embedded Credentials (Not Recommended)

For testing only, you can embed credentials in the build, but this is **NOT SECURE** for production:

```go
// DO NOT USE IN PRODUCTION
const embeddedCredentials = `{...service account JSON...}`
```

Instead, always use external credential files or environment-based secret management.

---

## Implementing OAuth 2.0 for Desktop App (Detailed Guide)

Since your organization blocks service account keys, here's a complete implementation guide for OAuth 2.0 user consent.

### Required Dependencies

Add to your `go.mod`:
```bash
go get golang.org/x/oauth2
go get golang.org/x/oauth2/google
```

### Complete Implementation Example

Create `pkg/auth/oauth.go`:

```go
package auth

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
)

type OAuthManager struct {
    config    *oauth2.Config
    tokenPath string
}

func NewOAuthManager(clientID, clientSecret string) *OAuthManager {
    homeDir, _ := os.UserHomeDir()
    tokenPath := filepath.Join(homeDir, ".config", "tal_assistant", "token.json")
    
    return &OAuthManager{
        config: &oauth2.Config{
            ClientID:     clientID,
            ClientSecret: clientSecret,
            Scopes: []string{
                "https://www.googleapis.com/auth/cloud-platform",
            },
            Endpoint:    google.Endpoint,
            RedirectURL: "http://localhost:8080/callback",
        },
        tokenPath: tokenPath,
    }
}

func (m *OAuthManager) GetClient(ctx context.Context) (*http.Client, error) {
    token, err := m.loadToken()
    if err != nil {
        // No valid token, need user consent
        token, err = m.getUserConsent(ctx)
        if err != nil {
            return nil, err
        }
        m.saveToken(token)
    }
    
    return m.config.Client(ctx, token), nil
}

func (m *OAuthManager) getUserConsent(ctx context.Context) (*oauth2.Token, error) {
    // Generate auth URL
    authURL := m.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
    
    // Open browser for user
    fmt.Printf("Opening browser for authentication...\n")
    fmt.Printf("If browser doesn't open, visit: %s\n", authURL)
    
    // Start local server to receive callback
    codeChan := make(chan string)
    errChan := make(chan error)
    
    server := &http.Server{Addr: ":8080"}
    http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
        code := r.URL.Query().Get("code")
        codeChan <- code
        w.Write([]byte("Authentication successful! You can close this window."))
    })
    
    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            errChan <- err
        }
    }()
    
    // Wait for code or error
    var code string
    select {
    case code = <-codeChan:
    case err := <-errChan:
        return nil, err
    case <-ctx.Done():
        return nil, ctx.Err()
    }
    
    server.Shutdown(ctx)
    
    // Exchange code for token
    token, err := m.config.Exchange(ctx, code)
    if err != nil {
        return nil, fmt.Errorf("failed to exchange token: %w", err)
    }
    
    return token, nil
}

func (m *OAuthManager) loadToken() (*oauth2.Token, error) {
    f, err := os.Open(m.tokenPath)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    
    token := &oauth2.Token{}
    err = json.NewDecoder(f).Decode(token)
    return token, err
}

func (m *OAuthManager) saveToken(token *oauth2.Token) error {
    os.MkdirAll(filepath.Dir(m.tokenPath), 0700)
    f, err := os.Create(m.tokenPath)
    if err != nil {
        return err
    }
    defer f.Close()
    
    return json.NewEncoder(f).Encode(token)
}
```

### Update STT Service

Modify `pkg/stt/sttservice.go` to use OAuth:

```go
import (
    speech "cloud.google.com/go/speech/apiv1"
    "google.golang.org/api/option"
    "yourproject/pkg/auth"
)

func NewSTTService(config *config.Config) (*STTService, error) {
    ctx := context.Background()
    
    var client *speech.Client
    var err error
    
    if config.OAuthClientID != "" {
        // Use OAuth
        oauthMgr := auth.NewOAuthManager(config.OAuthClientID, config.OAuthClientSecret)
        httpClient, err := oauthMgr.GetClient(ctx)
        if err != nil {
            return nil, fmt.Errorf("OAuth authentication failed: %w", err)
        }
        
        client, err = speech.NewClient(ctx, option.WithHTTPClient(httpClient))
    } else {
        // Fall back to service account or ADC
        client, err = speech.NewClient(ctx)
    }
    
    if err != nil {
        return nil, err
    }
    
    return &STTService{client: client}, nil
}
```

### Update Configuration

Add to `config/config.go`:

```go
type Config struct {
    // ... existing fields ...
    OAuthClientID     string `mapstructure:"OAUTH_CLIENT_ID"`
    OAuthClientSecret string `mapstructure:"OAUTH_CLIENT_SECRET"`
}
```

### Update app.env

```env
# OAuth Configuration (Recommended for production)
OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
OAUTH_CLIENT_SECRET=your-client-secret

# Legacy: Service Account (if admin override granted)
# GOOGLE_CREDENTIALS_PATH=./credentials/service-account-key.json

GOOGLE_PROJECT_ID=gen-lang-client-0165069269
GOOGLE_API_KEY=your-api-key-here
```

### User Experience

When users first launch the app:
1. Browser opens automatically
2. User logs in with their Google account
3. User grants permissions
4. Token is saved locally (`~/.config/tal_assistant/token.json`)
5. Subsequent launches use the cached token automatically

### Benefits of OAuth 2.0
- ✅ No service account keys to distribute
- ✅ Works with organization policies
- ✅ Users authenticate with their own credentials
- ✅ Tokens auto-refresh
- ✅ User can revoke access anytime
- ✅ Better audit trail (actions tied to specific users)
