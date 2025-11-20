# 🤖 AI Code Review Bot for GitLab

Automated GitLab Merge Request code review powered by Google Gemini AI. This bot analyzes code changes, identifies issues, and posts inline comments directly on your MRs.

## ✨ Features

- 🔍 **Intelligent Code Analysis**: Focuses on logic errors, security vulnerabilities, crash risks, and performance issues
- 📝 **Inline Comments**: Posts comments directly on the affected lines
- 🔄 **Smart Review Strategy**: Full review for new MRs, incremental review for updates
- 🔔 **Approval Notifications**: Notifies previous approvers when code changes after approval
- ⚡ **Real-time Processing**: Webhook-driven automation

## 📋 Prerequisites

- Node.js (v14 or higher)
- GitLab account with a project
- Google Gemini API key
- Server with public URL or tunneling solution (ngrok, Cloudflare Tunnel, etc.)

## 🚀 Quick Start

### 1. Clone and Install

```bash
git clone <your-repo-url>
cd ai-code-review
npm install
```

### 2. Configure Environment Variables

Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` with your credentials:

```env
# Server Configuration
PORT=5678

# GitLab Configuration
GITLAB_URL=https://gitlab.com
GITLAB_TOKEN=your_gitlab_personal_access_token_here

# Gemini AI Configuration
GEMINI_API_KEY=your_gemini_api_key_here
```

### 3. Get Your API Keys

#### GitLab Personal Access Token

1. Go to GitLab → **Settings** → **Access Tokens**
2. Create a new token with these scopes:
   - `api` (full API access)
   - `read_api`
   - `write_repository`
3. Copy the token to your `.env` file as `GITLAB_TOKEN`

#### Google Gemini API Key

1. Visit [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Click **Get API Key** or **Create API Key**
3. Copy the key to your `.env` file as `GEMINI_API_KEY`

### 4. Run the Server

Development mode (with auto-restart):

```bash
npm run dev
```

Production mode:

```bash
npm start
```

The server will start on `http://localhost:5678` (or your configured PORT).

## 🔗 GitLab Webhook Setup

### Option A: Using ngrok (for local development)

1. **Install ngrok**:

   ```bash
   brew install ngrok  # macOS
   # or download from https://ngrok.com/download
   ```

2. **Start ngrok tunnel**:

   ```bash
   ngrok http 5678
   ```

   Copy the HTTPS forwarding URL (e.g., `https://abc123.ngrok.io`)

### Option B: Using a Production Server

Deploy to your server and note the public URL (e.g., `https://your-domain.com`)

### Configure GitLab Webhook

1. Go to your GitLab project
2. Navigate to **Settings** → **Webhooks**
3. Add a new webhook with these settings:

   **URL**: `https://your-public-url/webhook/gitlab`

   - Replace with your ngrok URL or production server URL
   - Example: `https://abc123.ngrok.io/webhook/gitlab`

   **Trigger**: Select **Merge request events**

   **SSL verification**: Enable (recommended)

4. Click **Add webhook**
5. Test the webhook by clicking **Test** → **Merge request events**

## 📁 Project Structure

```
ai-code-review/
├── src/
│   ├── server.js          # Server entry point
│   ├── app.js             # Express app configuration
│   ├── config/
│   │   └── index.js       # Environment configuration
│   ├── ai/
│   │   └── gemini.js      # Gemini AI integration
│   ├── gitlab/
│   │   ├── api.js         # GitLab API client
│   │   ├── comments.js    # Comment posting logic
│   │   └── helpers.js     # Helper functions
│   └── webhook/
│       └── handler.js     # Webhook event processor
├── .env                   # Environment variables (not in git)
├── .env.example           # Environment template
├── package.json
└── README.md
```

## 🧪 Testing the Bot

1. Create a new branch in your GitLab project
2. Make some code changes
3. Create a Merge Request
4. The bot should automatically:
   - Analyze the code
   - Post inline comments on issues found
   - Update on subsequent pushes

## 🔧 Configuration

### Environment Variables

| Variable         | Description                  | Default            | Required |
| ---------------- | ---------------------------- | ------------------ | -------- |
| `PORT`           | Server port                  | 5678               | No       |
| `GITLAB_URL`     | GitLab instance URL          | https://gitlab.com | Yes      |
| `GITLAB_TOKEN`   | GitLab Personal Access Token | -                  | Yes      |
| `GEMINI_API_KEY` | Google Gemini API key        | -                  | Yes      |

### AI Model

The bot uses `gemini-2.0-flash` by default. You can change this in `src/config/index.js`.

## 📝 How It Works

1. **Webhook Trigger**: GitLab sends a webhook when MR events occur
2. **Event Filtering**: Bot checks if the event involves code changes
3. **Review Strategy**:
   - **Full Review**: For new MRs (analyzes all changes)
   - **Incremental Review**: For updates (analyzes only new changes)
4. **AI Analysis**: Gemini AI reviews the code for:
   - Logic errors
   - Security vulnerabilities
   - Crash risks
   - Performance issues
5. **Comment Posting**: Inline comments are posted on specific lines
6. **Approval Tracking**: Notifies previous approvers of new changes

## 🛠️ Troubleshooting

### Webhook Not Triggering

- Check that the webhook URL is publicly accessible
- Verify the webhook is enabled in GitLab settings
- Check server logs for incoming requests

### API Errors

- Verify your GitLab token has the correct permissions
- Check your Gemini API key is valid and has quota
- Ensure `GITLAB_URL` matches your GitLab instance

### Bot Not Commenting

- Check server logs for errors
- Verify the bot user has permissions to comment on MRs
- Ensure the GitLab token has `api` scope

## 📜 Scripts

- `npm start` - Run the server in production mode
- `npm run dev` - Run with auto-restart on file changes (requires nodemon)

## 🤝 Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details

## 🔗 Useful Links

- [GitLab Webhook Documentation](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html)
- [Google Gemini API Documentation](https://ai.google.dev/docs)
- [Express.js Documentation](https://expressjs.com/)
