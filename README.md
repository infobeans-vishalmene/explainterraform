

---

```markdown
# 🚀 Terraform AI Plan Explainer (`tf-explain`)

An automated DevOps CLI tool and GitHub Action that parses Terraform JSON plans, filters out structural noise, redacts sensitive keys, and uses LLMs via **OpenRouter** to post clear, human-readable impact reports on Pull Requests.

---

## 🏗️ Architecture


```

```
                ┌──────────────────────────────┐
                │  terraform show -json plan   │
                └──────────────┬───────────────┘
                               │
                               ▼

```

┌─────────────────────────────────────────────────────────────────────┐
│  tf-explain CLI Engine                                              │
│                                                                     │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐  │
│  │ 1. Data Reader  │───►│ 2. JSON Filter   │───►│ 3. Prompt Engine │  │
│  │   (File/Stdin)  │    │ (Diff & Redact) │    │  (Structured)   │  │
│  └─────────────────┘    └─────────────────┘    └────────┬────────┘  │
└─────────────────────────────────────────────────────────────────────┘
│
▼
┌────────────────────┐
│   OpenRouter API   │
│ (Llama 3.3 / Mistral)│
└──────────┬─────────┘
│
▼
┌────────────────────┐
│  PR Markdown Report│
└────────────────────┘

```

---

## ✨ Features

- 🧹 **Smart JSON Filtering:** Strips `no-op` / `read` resources and computes key attribute diffs to optimize LLM context usage.
- 🔒 **Privacy & Redaction:** Automatically masks sensitive attributes (`before_sensitive` / `after_sensitive`) before sending data to external APIs.
- ⚡ **Zero External Cloud Dependencies:** Can run entirely on local static files without requiring active cloud credentials.
- 🤖 **GitHub PR Automation:** Automatically analyzes Terraform PRs and updates a single sticky comment on every commit.

---

## 🛠️ Project Structure

```text
.
├── .github/
│   └── workflows/
│       └── tf-explain.yml    # GitHub Actions workflow
├── tf-explain/               # Go CLI Engine
│   ├── go.mod
│   ├── go.sum
│   ├── main.go               # Entrypoint & OpenRouter client
│   └── parser.go             # JSON filter & diff logic
├── mock_tfplan.json          # Synthetic test payload
├── .env.example
└── README.md

```

---

## 🚀 Local Quickstart

### Prerequisites

* [Go](https://go.dev/dl/) 1.22 or higher
* [OpenRouter API Key](https://openrouter.ai/)

### Setup

1. Clone the repository:
```bash
git clone [https://github.com/your-username/your-repo.git](https://github.com/your-username/your-repo.git)
cd your-repo/tf-explain

```


2. Initialize dependencies:
```bash
go mod download

```


3. Configure your environment variables:
Create a `.env` file in the `tf-explain/` directory:
```env
OPENROUTER_API_KEY=sk-or-v1-your-actual-api-key-here

```



### Usage

#### 1. Running against mock data

```bash
go run . ../mock_tfplan.json

```

#### 2. Running against real Terraform execution plans

```bash
# 1. Initialize Terraform
terraform init

# 2. Generate a binary plan file
terraform plan -out tfplan.binary

# 3. Pipe JSON output into tf-explain
terraform show -json tfplan.binary | go run .

```

#### 3. Building a static binary executable

```bash
# Build binary
go build -o tf-explain .

# Run binary
./tf-explain path/to/plan.json

```

---

## 🤖 GitHub Actions Setup

Automate pull request reviews by adding `tf-explain` to your repository pipelines.

### Step 1: Add API Secret

Add your OpenRouter key to GitHub Repository Secrets:

* **Settings** → **Secrets and variables** → **Actions** → **New repository secret**
* Name: `OPENROUTER_API_KEY`
* Value: `sk-or-v1-...`

### Step 2: Add Workflow (`.github/workflows/tf-explain.yml`)

```yaml
name: "Terraform AI Plan Explainer"

on:
  pull_request:
    types: [opened, synchronize, reopened]
    paths:
      - "**.tf"

jobs:
  explain-plan:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write

    steps:
      - name: Checkout Code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: 'tf-explain/go.mod'

      - name: Setup Terraform
        uses: hashicorp/setup-terraform@v3
        with:
          terraform_wrapper: false

      - name: Terraform Init & Plan
        run: |
          terraform init
          terraform plan -out tfplan.binary
          terraform show -json tfplan.binary > plan.json

      - name: Generate AI Explanation
        env:
          OPENROUTER_API_KEY: ${{ secrets.OPENROUTER_API_KEY }}
        run: |
          jq empty plan.json || (echo "ERROR: plan.json is corrupt" && exit 1)
          cd tf-explain
          go build -o tf-explain-bin .
          ./tf-explain-bin ../plan.json > ../ai_comment.md

      - name: Post Comment to PR
        uses: thollander/actions-comment-pull-request@v3
        with:
          file-path: ai_comment.md
          comment-tag: "tf-explain-analysis"

```

---

