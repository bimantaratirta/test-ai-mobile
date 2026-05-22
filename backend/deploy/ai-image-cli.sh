#!/usr/bin/env bash
# AI Image API quick test — share with collaborators / beta testers.
#
# Hits the live backend at ai.ashakita.net via curl: signs in to Supabase,
# redeems the invite once, uploads a photo, kicks off a generation job,
# polls until done, and opens the result.
#
# Usage:
#   # Coffee shop redesign (default; uses backend's coffee-shop wrapper):
#   ./ai-image-cli.sh path/to/empty-room.jpg "Scandinavian, light oak, plants"
#
#   # Anything else — prefix prompt with "RAW:" to bypass the wrapper.
#   # Examples:
#   ./ai-image-cli.sh path/to/face.jpg     "RAW: turn this person into a Renaissance oil painting"
#   ./ai-image-cli.sh path/to/product.jpg  "RAW: place this watch on a marble surface, soft studio light"
#   ./ai-image-cli.sh path/to/sketch.jpg   "RAW: convert this rough sketch into a polished isometric illustration"
#
# Prerequisites:
#   - curl, jq (brew install jq / apt install jq)
#   - macOS or Linux. Auto-opens result on macOS; on Linux it prints the URL.
#
# Rate limits in effect (per the backend's beta config):
#   - 3 generations / day per user, 15 / week
#   - 2 concurrent jobs per user
#   - Global cost cap (across all users) of $8/day. Past that, /generate
#     returns 503 until midnight UTC.
set -euo pipefail

# ============================================================
# CREDENTIALS — filled in for the user you've been given.
# Change these to your own creds if you have a different account.
# ============================================================
EMAIL="${EMAIL:-friend1@example.com}"
PASSWORD="${PASSWORD:-Friend2026!}"
INVITE="${INVITE:-FRIEND1}"

# ============================================================
# CONSTANTS — leave alone unless backend moved.
# ============================================================
API="${API:-https://ai.ashakita.net}"
SUPABASE_URL="${SUPABASE_URL:-https://qzclcljyibbtdkuixarj.supabase.co}"
SUPABASE_ANON_KEY="${SUPABASE_ANON_KEY:-sb_publishable_Bt_RrPOKIdt6Rwom5yy1sw_OyWJhFfx}"
STYLE_PRESET="${STYLE_PRESET:-scandinavian}"  # ignored when prompt starts with RAW:
MODE="${MODE:-realistic}"                       # realistic | inspirational

if [ $# -lt 2 ]; then
  echo "Usage: $0 path/to/photo.jpg \"your prompt\""
  echo "       prefix prompt with 'RAW:' to skip the coffee-shop wrapper"
  exit 1
fi

IMAGE_PATH="$1"
PROMPT="$2"

if [ ! -f "$IMAGE_PATH" ]; then
  echo "❌ File not found: $IMAGE_PATH" >&2
  exit 1
fi

command -v jq >/dev/null 2>&1 || {
  echo "❌ jq is required. Install with: brew install jq  (or: apt install jq)" >&2
  exit 1
}

# ============================================================
# 1. Sign in to Supabase, get JWT.
# ============================================================
echo "==> 1/6  Signing in to Supabase…"
SIGNIN=$(curl -s -X POST "$SUPABASE_URL/auth/v1/token?grant_type=password" \
  -H "apikey: $SUPABASE_ANON_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
JWT=$(echo "$SIGNIN" | jq -r .access_token)
if [ "$JWT" = "null" ] || [ -z "$JWT" ]; then
  echo "❌ Sign-in failed:" >&2
  echo "$SIGNIN" | jq . >&2
  exit 1
fi
echo "    JWT acquired ✓"

# ============================================================
# 2. Redeem invite — first run only; subsequent runs return
#    "already used" which we ignore.
# ============================================================
echo "==> 2/6  Redeeming invite (no-op if already done)…"
REDEEM=$(curl -s -X POST "$API/redeem-invite" \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d "{\"code\":\"$INVITE\"}")
echo "    $(echo "$REDEEM" | jq -c .)"

# ============================================================
# 3. Get a presigned S3 PUT URL.
# ============================================================
echo "==> 3/6  Asking backend for a presigned upload URL…"
PRESIGN=$(curl -s -X POST "$API/uploads/presign" \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d '{"content_type":"image/jpeg"}')
UPLOAD_URL=$(echo "$PRESIGN" | jq -r .upload_url)
PUBLIC_URL=$(echo "$PRESIGN" | jq -r .public_url)
if [ "$UPLOAD_URL" = "null" ] || [ -z "$UPLOAD_URL" ]; then
  echo "❌ Presign failed:" >&2
  echo "$PRESIGN" | jq . >&2
  exit 1
fi
echo "    Public URL will be: $PUBLIC_URL"

# ============================================================
# 4. Upload the photo via PUT.
# ============================================================
echo "==> 4/6  Uploading photo ($(du -h "$IMAGE_PATH" | cut -f1))…"
UPLOAD_HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$UPLOAD_URL" \
  -H "Content-Type: image/jpeg" \
  --data-binary "@$IMAGE_PATH")
if [ "$UPLOAD_HTTP" != "200" ]; then
  echo "❌ Upload failed with HTTP $UPLOAD_HTTP" >&2
  exit 1
fi
echo "    Uploaded ✓"

# ============================================================
# 5. Submit generation job.
# ============================================================
echo "==> 5/6  Submitting generation job…"
BODY=$(jq -nc \
  --arg url    "$PUBLIC_URL" \
  --arg mode   "$MODE" \
  --arg prompt "$PROMPT" \
  --arg preset "$STYLE_PRESET" \
  '{input_image_url:$url, mode:$mode, prompt:$prompt, style_preset:$preset}')
JOB=$(curl -s -X POST "$API/generate" \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d "$BODY")
JOB_ID=$(echo "$JOB" | jq -r .job_id)
if [ "$JOB_ID" = "null" ] || [ -z "$JOB_ID" ]; then
  echo "❌ Generate request rejected:" >&2
  echo "$JOB" | jq . >&2
  exit 1
fi
echo "    Job $JOB_ID queued, polling every 5s…"

# ============================================================
# 6. Poll for completion. Flex tier can take 30–90s.
# ============================================================
echo "==> 6/6  Waiting for result…"
START=$(date +%s)
for i in $(seq 1 60); do
  R=$(curl -s "$API/jobs/$JOB_ID" -H "Authorization: Bearer $JWT")
  STATUS=$(echo "$R" | jq -r .status)
  ELAPSED=$(( $(date +%s) - START ))
  printf "    [%2ds] %s\n" "$ELAPSED" "$STATUS"
  case "$STATUS" in
    completed)
      OUT=$(echo "$R" | jq -r .output_image_url)
      GEN_MS=$(echo "$R" | jq -r .generation_ms)
      echo ""
      echo "✅ Done in ${GEN_MS}ms (server-side)."
      echo "   Result: $OUT"
      if [ "$(uname)" = "Darwin" ] && command -v open >/dev/null; then
        open "$OUT"
      fi
      exit 0
      ;;
    failed)
      ERR=$(echo "$R" | jq -r .error_message)
      echo ""
      echo "❌ Failed: $ERR" >&2
      exit 1
      ;;
  esac
  sleep 5
done

echo "⏱ Timed out after 5 minutes. Inspect manually:"
echo "   curl '$API/jobs/$JOB_ID' -H 'Authorization: Bearer $JWT' | jq ."
exit 1
