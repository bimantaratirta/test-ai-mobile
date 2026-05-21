#!/usr/bin/env bash
# Manual deploy script for Cloud Run.
# Prerequisites: gcloud CLI installed and authenticated (gcloud auth login).
# Required env vars (set in your shell or .env, NOT committed):
#   GCP_PROJECT  - your Google Cloud project ID
#   GCP_REGION   - asia-southeast1 (Singapore) recommended
#   AR_REPO      - Artifact Registry repo name (default: ai-image)
set -euo pipefail

: "${GCP_PROJECT:?GCP_PROJECT not set}"
: "${GCP_REGION:=asia-southeast1}"
: "${AR_REPO:=ai-image}"

SERVICE=ai-image-backend
IMAGE="${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT}/${AR_REPO}/${SERVICE}:$(git rev-parse --short HEAD)"

echo "==> Building Docker image: ${IMAGE}"
docker build --platform linux/amd64 -t "$IMAGE" .

echo "==> Pushing to Artifact Registry"
gcloud auth configure-docker "${GCP_REGION}-docker.pkg.dev" --quiet
docker push "$IMAGE"

echo "==> Deploying to Cloud Run (region: ${GCP_REGION})"
gcloud run deploy "$SERVICE" \
  --image "$IMAGE" \
  --region "$GCP_REGION" \
  --platform managed \
  --allow-unauthenticated \
  --port 8080 \
  --memory 512Mi \
  --cpu 1 \
  --min-instances 0 \
  --max-instances 10 \
  --timeout 300 \
  --concurrency 80 \
  --set-env-vars "ENV=production" \
  --update-secrets "DATABASE_URL=database-url:latest,\
SUPABASE_URL=supabase-url:latest,\
SUPABASE_SERVICE_ROLE_KEY=supabase-service-role-key:latest,\
SUPABASE_JWT_SECRET=supabase-jwt-secret:latest,\
R2_ACCOUNT_ID=r2-account-id:latest,\
R2_ACCESS_KEY_ID=r2-access-key-id:latest,\
R2_SECRET_ACCESS_KEY=r2-secret-access-key:latest,\
R2_BUCKET=r2-bucket:latest,\
R2_PUBLIC_BASE_URL=r2-public-base-url:latest,\
GEMINI_API_KEY=gemini-api-key:latest,\
REPLICATE_API_TOKEN=replicate-api-token:latest,\
REPLICATE_FLUX_VERSION=replicate-flux-version:latest,\
SENTRY_DSN=sentry-dsn:latest"

echo "==> Done. Service URL:"
gcloud run services describe "$SERVICE" --region "$GCP_REGION" --format "value(status.url)"
