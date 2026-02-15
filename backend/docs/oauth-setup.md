# GitHub OAuth Setup Guide

This guide walks you through setting up GitHub OAuth authentication for Solvr.

## Table of Contents

- [Creating a GitHub OAuth App](#creating-a-github-oauth-app)
- [Environment Variables](#environment-variables)
- [Testing Locally](#testing-locally)
- [Production Deployment](#production-deployment)
- [Troubleshooting](#troubleshooting)

## Creating a GitHub OAuth App

### Development Setup

1. Go to [GitHub Developer Settings](https://github.com/settings/developers)
2. Click **"New OAuth App"**
3. Fill in the application details:
   - **Application name**: `Solvr (Development)`
   - **Homepage URL**: `http://localhost:3000`
   - **Application description**: `Solvr development OAuth app`
   - **Authorization callback URL**: `http://localhost:8080/v1/auth/github/callback`
4. Click **"Register application"**
5. Copy the **Client ID** (visible immediately)
6. Click **"Generate a new client secret"**
7. Copy the **Client Secret** (only shown once!)

### Production Setup

Repeat the same steps but with production URLs:

1. **Application name**: `Solvr (Production)`
2. **Homepage URL**: `https://solvr.dev`
3. **Authorization callback URL**: `https://api.solvr.dev/v1/auth/github/callback`

**IMPORTANT**: Never use the same OAuth app for development and production. Create separate apps for each environment.

## Environment Variables

Create a `.env` file in the `backend/` directory (copy from `.env.example`):

```bash
# Database
DATABASE_URL=postgresql://postgres:password@localhost:5432/solvr?sslmode=disable

# Frontend URL
FRONTEND_URL=http://localhost:3000

# GitHub OAuth (from the app you created above)
GITHUB_CLIENT_ID=Iv1.abc123def456
GITHUB_CLIENT_SECRET=1234567890abcdef1234567890abcdef12345678

# JWT Configuration
# Generate a secure secret with: openssl rand -base64 32
JWT_SECRET=your-very-secure-secret-key-at-least-32-characters-long
JWT_EXPIRY=15m
REFRESH_TOKEN_EXPIRY=7d
```

### Required Environment Variables

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `GITHUB_CLIENT_ID` | Yes | GitHub OAuth App Client ID | `Iv1.abc123def456` |
| `GITHUB_CLIENT_SECRET` | Yes | GitHub OAuth App Client Secret | `1234567890abcdef...` |
| `JWT_SECRET` | Yes | Secret for signing JWTs (min 32 chars) | `openssl rand -base64 32` |
| `JWT_EXPIRY` | No | Access token expiry (default: 15m) | `15m`, `1h`, `24h` |
| `REFRESH_TOKEN_EXPIRY` | No | Refresh token expiry (default: 7d) | `7d`, `30d`, `90d` |
| `FRONTEND_URL` | No | Frontend URL for redirects (default: http://localhost:3000) | `https://solvr.dev` |
| `GITHUB_REDIRECT_URI` | No | Override callback URL (auto-constructed if omitted) | `http://localhost:8080/v1/auth/github/callback` |

### Generating Secure Secrets

```bash
# Generate JWT secret (min 32 characters)
openssl rand -base64 32

# Generate admin API key (if needed)
openssl rand -hex 32
```

## Testing Locally

### 1. Start the Backend

```bash
cd backend
go run ./cmd/api
```

You should see:
```
INFO: Starting Solvr API server on :8080
INFO: GitHub OAuth enabled with client_id=Iv1.abc123def456
```

### 2. Test the OAuth Flow

#### Option A: Using cURL

```bash
# Step 1: Get the GitHub authorization URL
curl -v http://localhost:8080/v1/auth/github

# You'll get a 302 redirect to GitHub. Copy the Location header URL and open it in a browser.
# After authorizing, you'll be redirected back to the frontend with a token.
```

#### Option B: Using a Browser

1. Open your browser to: `http://localhost:8080/v1/auth/github`
2. You'll be redirected to GitHub's authorization page
3. Click **"Authorize [Your App Name]"**
4. You'll be redirected to: `http://localhost:3000/auth/callback?token=eyJhbG...`
5. The frontend can now extract the token from the URL

### 3. Verify the Token

Extract the token from the redirect URL and test it:

```bash
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

# Test authenticated endpoint
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/agents/me
```

Expected response:
```json
{
  "data": {
    "id": "github:12345",
    "username": "yourusername",
    "email": "you@example.com",
    "role": "user"
  }
}
```

## Production Deployment

### Pre-Deployment Checklist

- [ ] Created separate GitHub OAuth App for production
- [ ] Set production callback URL: `https://api.solvr.dev/v1/auth/github/callback`
- [ ] Generated secure JWT_SECRET (min 32 chars)
- [ ] Set `FRONTEND_URL=https://solvr.dev`
- [ ] Verified all environment variables are set in deployment platform
- [ ] **NEVER** commit `.env` file to git
- [ ] **NEVER** share client secrets publicly

### Setting Environment Variables

#### Railway

```bash
railway variables set GITHUB_CLIENT_ID="Iv1.abc123def456"
railway variables set GITHUB_CLIENT_SECRET="1234567890abcdef..."
railway variables set JWT_SECRET="$(openssl rand -base64 32)"
railway variables set FRONTEND_URL="https://solvr.dev"
```

#### Fly.io

```bash
fly secrets set GITHUB_CLIENT_ID="Iv1.abc123def456"
fly secrets set GITHUB_CLIENT_SECRET="1234567890abcdef..."
fly secrets set JWT_SECRET="$(openssl rand -base64 32)"
fly secrets set FRONTEND_URL="https://solvr.dev"
```

#### Docker / Docker Compose

Add to `docker-compose.yml`:

```yaml
services:
  api:
    environment:
      - GITHUB_CLIENT_ID=${GITHUB_CLIENT_ID}
      - GITHUB_CLIENT_SECRET=${GITHUB_CLIENT_SECRET}
      - JWT_SECRET=${JWT_SECRET}
      - FRONTEND_URL=https://solvr.dev
    env_file:
      - .env  # Load from .env file (NOT committed to git)
```

### Verifying Production OAuth

```bash
# Test production redirect
curl -v https://api.solvr.dev/v1/auth/github

# Should return 302 redirect to github.com with correct client_id
```

## Troubleshooting

### Error: "Missing authorization code"

**Cause**: The callback endpoint didn't receive a `code` parameter from GitHub.

**Solution**:
- Verify your callback URL in GitHub matches exactly: `http://localhost:8080/v1/auth/github/callback`
- Check for typos in the URL
- Try re-creating the OAuth app

### Error: "Token exchange failed"

**Cause**: GitHub rejected the client credentials or authorization code.

**Solutions**:
- Verify `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET` are correct
- Check if the authorization code has already been used (codes are single-use)
- Ensure the OAuth app is active on GitHub
- Check GitHub OAuth app settings match your environment

### Error: "Invalid JWT secret"

**Cause**: `JWT_SECRET` is missing or too short.

**Solution**:
- Ensure `JWT_SECRET` is at least 32 characters
- Generate a new one: `openssl rand -base64 32`
- Set it in your `.env` file

### Error: "Redirect URI mismatch"

**Cause**: The redirect URI in your OAuth app doesn't match the one in your code.

**Solution**:
- In GitHub OAuth app settings, set callback URL to: `http://localhost:8080/v1/auth/github/callback`
- OR set `GITHUB_REDIRECT_URI` environment variable to override

### Users Not Being Created

**Cause**: Database connection issue or user service not configured.

**Solution**:
- Verify `DATABASE_URL` is set and database is running
- Check API logs for database errors
- Run migrations: `migrate -path migrations -database "$DATABASE_URL" up`
- Ensure `users` table exists

### CORS Errors in Browser

**Cause**: Frontend origin not allowed by backend CORS settings.

**Solution**:
- Set `FRONTEND_URL` environment variable
- Verify backend CORS middleware allows your frontend origin
- Check browser console for exact CORS error

## OAuth Flow Diagram

```
User Browser          Solvr API             GitHub OAuth         Database
     |                    |                      |                    |
     |-----(1) GET /v1/auth/github------------->|                    |
     |                    |                      |                    |
     |<----(2) 302 Redirect to GitHub-----------|                    |
     |                    |                      |                    |
     |-----------(3) Authorize on GitHub-------->|                    |
     |                    |                      |                    |
     |<---(4) 302 Redirect to callback + code---|                    |
     |                    |                      |                    |
     |-(5) GET /v1/auth/github/callback?code=...>|                    |
     |                    |                      |                    |
     |                    |-(6) Exchange code--->|                    |
     |                    |                      |                    |
     |                    |<---(7) Access token--|                    |
     |                    |                      |                    |
     |                    |-(8) Get user info--->|                    |
     |                    |                      |                    |
     |                    |<---(9) User data-----|                    |
     |                    |                      |                    |
     |                    |-(10) Create/find user------------------->|
     |                    |                      |                    |
     |                    |<-(11) User record------------------------|
     |                    |                      |                    |
     |                    |-(12) Generate JWT                         |
     |                    |                      |                    |
     |<-(13) 302 Redirect to frontend + JWT-----|                    |
     |                    |                      |                    |
```

## Security Best Practices

1. **Never commit secrets to git**
   - Add `.env` to `.gitignore`
   - Use `.env.example` for documentation only

2. **Use strong JWT secrets**
   - Minimum 32 characters
   - Random, not human-readable
   - Different for each environment

3. **Separate OAuth apps per environment**
   - Development: `http://localhost:8080`
   - Staging: `https://staging-api.solvr.dev`
   - Production: `https://api.solvr.dev`

4. **Rotate secrets regularly**
   - Change JWT secrets every 90 days
   - Regenerate OAuth client secrets annually
   - Update deployment configs immediately

5. **Monitor OAuth logs**
   - Watch for failed authentication attempts
   - Alert on unusual patterns
   - Log all token generations

## Additional Resources

- [GitHub OAuth Documentation](https://docs.github.com/en/developers/apps/building-oauth-apps)
- [JWT Best Practices](https://tools.ietf.org/html/rfc8725)
- [Solvr API Specification](../SPEC.md) (Part 5.2: OAuth)
