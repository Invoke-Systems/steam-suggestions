# Deploy secrets (GitHub Actions)

Add these under **Settings → Secrets and variables → Actions** on
`Invoke-Systems/steam-suggestions`:

| Secret | Required | Purpose |
|--------|----------|---------|
| `STEAM_API_KEY` | yes | Steam Web API key for the host |
| `SHOULDI_PLAY_SSH_PRIVATE_KEY` | yes | Private key matching the pubkey on the nanode (`matth`, port 2222). Can be the same key as `HOSTING_SSH_PRIVATE_KEY` in tf-invoke-systems-linode if that pubkey is in the instance `terraform.tfvars`. |
| `SHOULDI_PLAY_HOST` | yes | Nanode IPv4 (set after `shouldiplay.co/01-instance` apply) |
| `ITAD_API_KEY` | no | IsThereAnyDeal key for worker historical lows |
| `UMAMI_WEBSITE_ID` | no | Analytics site id; leaves tracker off if unset |

Also grant the default `GITHUB_TOKEN` permission to write packages (repo
Settings → Actions → General → Workflow permissions → Read and write), so
the workflow can push to `ghcr.io/invoke-systems/steam-suggestions`.

## Infra secrets (tf-invoke-systems-linode)

Reuse existing Linode/Object Storage secrets. No new repo secrets required
for the nanode itself if the committed SSH pubkey already matches your
deploy key.
