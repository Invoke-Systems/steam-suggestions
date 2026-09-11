# Deploy secrets

App image publish lives in this repo. Host deploy (SSH + Steam/ITAD/Umami
env) lives in [tf-invoke-systems-linode](https://github.com/Invoke-Systems/tf-invoke-systems-linode)
— see `shouldiplay.co/` and `.github/workflows/shouldiplay-deploy.yml` there.

## This repo (`Invoke-Systems/steam-suggestions`)

| Secret | Required | Purpose |
|--------|----------|---------|
| `TF_DEPLOY_PAT` | no | PAT with `repo` scope on `tf-invoke-systems-linode`. When set, `publish-image` dispatches `shouldiplay-deploy` after pushing to GHCR. Without it, publish still works; deploy manually from the TF repo. |

Also grant the default `GITHUB_TOKEN` permission to write packages (repo
Settings → Actions → General → Workflow permissions → Read and write), so
the workflow can push to `ghcr.io/invoke-systems/steam-suggestions`.

## Infra repo (`Invoke-Systems/tf-invoke-systems-linode`)

Reuse existing Linode/SSH secrets. Add app runtime secrets there:

| Secret | Required | Purpose |
|--------|----------|---------|
| `STEAM_API_KEY` | yes | Steam Web API key on the nanode |
| `ITAD_API_KEY` | no | IsThereAnyDeal key for worker historical lows |
| `UMAMI_WEBSITE_ID` | no | Analytics site id; leaves tracker off if unset |
| `HOSTING_SSH_PRIVATE_KEY` | yes | Private key for `matth@` on port 2222 (same key as shared hosting if that pubkey is on the nanode) |
| `LINODE_INSTANCES_API_KEY` | yes | Read instance IP via Terraform state |
| `LINODE_OBJ_ACCESS_KEY` / `LINODE_OBJ_SECRET_KEY` | yes | Read `shouldiplay-co-tfstate` |

No `SHOULDI_PLAY_HOST` — deploy resolves the nanode IP from
`shouldiplay.co/01-instance` Terraform output.
