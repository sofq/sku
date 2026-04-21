# Data-validate workflow runbook

Maintainer guide for `.github/workflows/data-validate.yml` — the weekly
(Mondays 04:00 UTC + `workflow_dispatch`) cross-check that re-fetches a
stratified sample of SKUs from each upstream provider, compares against the
published shard, and files a `catalog-drift` issue on >1% price divergence.
Independent of `data-daily.yml`: a failing validate run does **not** roll
back a published release, it just alerts.

## Auth model

| Provider | Auth path | Why not anonymous |
|---|---|---|
| AWS | Short-lived OIDC → IAM role `sku-validator` → SigV4 `pricing:GetProducts` | Query API requires SigV4. Bulk JSON is anonymous but the daily ingest already reads it — cross-checking against the same source catches fewer parser bugs. |
| GCP | Short-lived OIDC → Workload Identity Federation → service account `sku-validator@<project>.iam` → `cloudbilling.googleapis.com/v1/services/{sid}/skus` | Cloud Billing Catalog API requires a bearer token. WIF avoids the long-lived `GCP_BILLING_API_KEY` that `data-daily.yml` uses. |
| Azure | Anonymous `prices.azure.com/api/retail/prices` | Azure retail prices are public. |
| OpenRouter | Anonymous `openrouter.ai/api/v1/models/{id}/endpoints` | Public. |

Both federated identities are **read-only** and scoped to `repo:sofq/sku`.
No long-lived credentials live in repo secrets for this workflow.

## Repo variables required (non-sensitive — variables, not secrets)

| Variable | Example | Owner |
|---|---|---|
| `AWS_VALIDATE_ROLE_ARN` | `arn:aws:iam::123456789012:role/sku-validator` | Maintainer |
| `GCP_WIF_PROVIDER` | `projects/123/locations/global/workloadIdentityPools/github/providers/sofq-sku` | Maintainer |
| `GCP_VALIDATE_SA` | `sku-validator@my-project.iam.gserviceaccount.com` | Maintainer |

Until these are populated, `workflow_dispatch` for AWS or GCP shards will
fail on the auth step. Azure- and OpenRouter-only dispatches still succeed.

## One-time provisioning — AWS

1. Pick (or create) a dedicated AWS account with nothing else running in it.
   `pricing:GetProducts` itself is free; the isolation is for blast-radius.
2. Create the role via AWS CLI (substitute your account ID):

   ```bash
   cat > trust.json <<'EOF'
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Principal": { "Federated": "arn:aws:iam::AWS_ACCOUNT_ID:oidc-provider/token.actions.githubusercontent.com" },
         "Action": "sts:AssumeRoleWithWebIdentity",
         "Condition": {
           "StringEquals": {
             "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
           },
           "StringLike": {
             "token.actions.githubusercontent.com:sub": [
               "repo:sofq/sku:ref:refs/heads/main",
               "repo:sofq/sku:pull_request"
             ]
           }
         }
       }
     ]
   }
   EOF

   aws iam create-role \
     --role-name sku-validator \
     --assume-role-policy-document file://trust.json

   aws iam attach-role-policy \
     --role-name sku-validator \
     --policy-arn arn:aws:iam::aws:policy/AWSPriceListServiceFullAccess
   ```

3. If the GitHub OIDC provider is not yet registered in the account:

   ```bash
   aws iam create-open-id-connect-provider \
     --url https://token.actions.githubusercontent.com \
     --client-id-list sts.amazonaws.com \
     --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1
   ```

4. Copy the role ARN into repo variables:

   ```bash
   gh variable set AWS_VALIDATE_ROLE_ARN \
     --body "arn:aws:iam::AWS_ACCOUNT_ID:role/sku-validator"
   ```

5. Verify: dispatch `data-validate.yml` for a single AWS shard:

   ```bash
   gh workflow run data-validate.yml -F shards=aws-s3
   gh run watch
   ```

   Expected: green. If the auth step fails, re-check the `sub` claim in
   the trust policy matches `repo:sofq/sku:ref:refs/heads/<branch>` or
   `repo:sofq/sku:pull_request`.

## One-time provisioning — GCP

1. Pick (or create) a dedicated GCP project. The Cloud Billing Catalog API
   is free; isolate for blast-radius.
2. Enable APIs:

   ```bash
   gcloud services enable \
     cloudbilling.googleapis.com \
     iamcredentials.googleapis.com \
     --project "$PROJECT_ID"
   ```

3. Create the service account + grant read-only billing:

   ```bash
   gcloud iam service-accounts create sku-validator \
     --project "$PROJECT_ID" \
     --display-name "sku catalog validator"

   SA="sku-validator@${PROJECT_ID}.iam.gserviceaccount.com"

   gcloud projects add-iam-policy-binding "$PROJECT_ID" \
     --member "serviceAccount:${SA}" \
     --role   roles/billing.viewer

   gcloud projects add-iam-policy-binding "$PROJECT_ID" \
     --member "serviceAccount:${SA}" \
     --role   roles/serviceusage.serviceUsageConsumer
   ```

4. Create the Workload Identity Pool + GitHub provider:

   ```bash
   gcloud iam workload-identity-pools create github \
     --project "$PROJECT_ID" \
     --location global \
     --display-name "GitHub Actions"

   gcloud iam workload-identity-pools providers create-oidc sofq-sku \
     --project "$PROJECT_ID" \
     --location global \
     --workload-identity-pool github \
     --issuer-uri "https://token.actions.githubusercontent.com" \
     --attribute-mapping "google.subject=assertion.sub,attribute.repository=assertion.repository" \
     --attribute-condition "assertion.repository=='sofq/sku'"
   ```

5. Bind the service account to the pool so GitHub Actions can impersonate it:

   ```bash
   POOL_ID=$(gcloud iam workload-identity-pools describe github \
     --project "$PROJECT_ID" --location global --format 'value(name)')

   gcloud iam service-accounts add-iam-policy-binding "$SA" \
     --project "$PROJECT_ID" \
     --role roles/iam.workloadIdentityUser \
     --member "principalSet://iam.googleapis.com/${POOL_ID}/attribute.repository/sofq/sku"
   ```

6. Copy the provider resource name + SA email into repo variables:

   ```bash
   PROVIDER=$(gcloud iam workload-identity-pools providers describe sofq-sku \
     --project "$PROJECT_ID" --location global \
     --workload-identity-pool github --format 'value(name)')

   gh variable set GCP_WIF_PROVIDER --body "$PROVIDER"
   gh variable set GCP_VALIDATE_SA --body "$SA"
   ```

7. Verify: dispatch `data-validate.yml` for a single GCP shard:

   ```bash
   gh workflow run data-validate.yml -F shards=gcp-gce
   gh run watch
   ```

## Rotation policy

Review quarterly:
- IAM role + attached policies (confirm still read-only).
- WIF provider + SA bindings (confirm principalSet condition still scopes to `sofq/sku`).
- No access keys or service-account JSON keys anywhere. If either appears, rotate and remove.

## What breaks if OIDC is misconfigured

The `configure-aws-credentials@v4` / `google-github-actions/auth@v2` step
fails. The matrix cell for that shard fails; other cells keep running
(`fail-fast: false`). **No release is affected** — `data-daily.yml` runs
independently and doesn't consult `data-validate.yml`.

Recovery: fix the trust policy or WIF condition, re-dispatch.

## Triaging a `catalog-drift` issue

Filed automatically by `data-validate.yml` on matrix-cell failure. Body
links to the run + artifact `validate-<shard>` (the per-shard JSON report).

Decision tree:

1. **Download the artifact**; inspect `drift_records`. Each record has
   `sku_id`, `catalog_amount`, `upstream_amount`, `delta_pct`, `source`.
2. **Real upstream price change?** (common after AWS price revisions).
   - Verify upstream by hand: `aws pricing get-products --service-code AmazonEC2 --filters ...`.
   - If upstream has indeed moved, the shard will pick it up on the next
     `data-daily` run. Close the issue with a link to the next release.
3. **Upstream unchanged, catalog wrong?** (parser bug).
   - Disable the `data-daily.yml` cron temporarily (comment out the
     `schedule` block; merge; re-enable after fix).
   - Investigate the relevant `pipeline/ingest/<shard>.py`; add a
     regression test fixture in `pipeline/tests/`.
   - Push the fix; force a `workflow_dispatch` of `data-daily.yml` with
     `force_baseline=true` to republish.
4. **Validator false positive** (e.g. unit-of-measure mismatch that
   doesn't matter economically).
   - Patch `pipeline/validate/<provider>.py` or the specific filter in
     `pipeline/validate/sampler.py`.
   - Add a regression test.

## EC2 offline cross-check (Vantage)

For `aws-ec2`, the workflow additionally downloads
`vantage-sh/ec2instances.info/www/instances.json` at run time and joins
against the shard on `(instance_type, region, os=linux, tenancy=shared)`.
Drift >1% flags a record under `vantage_drift` in the report. This is a
zero-credential cross-check — useful even when the OIDC path is down.
