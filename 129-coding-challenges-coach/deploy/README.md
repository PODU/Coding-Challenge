# Cloud deployment (Steps 6 & 7)

These steps require a Google Cloud account and are not automatable from this
repo; everything below is the exact procedure, and `agent_config.yaml` is the
config the mock audit (Step 8) validates locally.

## Step 6 — deploy to Agent Runtime

```bash
# once
gcloud auth login
gcloud config set project <PROJECT_ID>
pip install google-adk

# deploy (from the repo root)
adk deploy agent_engine \
  --project <PROJECT_ID> \
  --region europe-west1 \
  --staging_bucket gs://<BUCKET> \
  src/coach
```

The runtime exposes a public HTTPS endpoint and serves Gemini models directly
— no API key ships with the code (`AnthropicClient`/`MockLLMClient` are the
local dev clients; in the runtime the model binding comes from
`agent_config.yaml`). Continuous deployment: connect the repo in Cloud Build
with a trigger on `main` that re-runs the command above.

## Step 7 — Agent Identity (SPIFFE)

```bash
gcloud iam workload-identity-pools create coach-pool --location=global
adk deploy agent_engine ... --agent_identity enabled
```

- The agent authenticates to every Google API with short-lived SPIFFE SVID
  tokens (`spiffe_id` in `agent_config.yaml`); there are no static
  credentials to rotate or leak.
- Cloud Audit Logs record every action under the agent principal
  `spiffe://codingchallenges.example.org/agent/coach`.

## Step 8 — audit it

```bash
python -m coach.audit deploy/agent_config.yaml src
```

Four checks: no static credentials in the tree, least-privilege IAM roles,
encrypted state storage, debug endpoints disabled (probe must see 404/401).
Exit code 0 only when all four pass.
