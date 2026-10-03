#!/usr/bin/env bash
# Copy edge secrets from SSM into the CloudFront Key-Value Store.
set -euo pipefail

ENVIRONMENT="${1:?usage: $0 <dev|prod> [kvs-arn]}"
AWS_REGION="${AWS_REGION:-ap-south-1}"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TF_DIR="${TF_DIR:-$REPO_ROOT/infra/environments/dev}"
KVS_ARN="${2:-$(terraform -chdir="$TF_DIR" output -raw stub_origin_kvs_arn)}"

ETAG="$(aws cloudfront-keyvaluestore describe-key-value-store \
  --kvs-arn "$KVS_ARN" --query ETag --output text)"

for key in ADMISSION_SECRET SESSION_SECRET; do
  value="$(aws ssm get-parameter \
    --name "/virtual-queue/$ENVIRONMENT/$key" \
    --with-decryption \
    --query Parameter.Value \
    --output text \
    --region "$AWS_REGION")"

  aws cloudfront-keyvaluestore put-key \
    --kvs-arn "$KVS_ARN" \
    --key "$key" \
    --value "$value" \
    --if-match "$ETAG" >/dev/null

  ETAG="$(aws cloudfront-keyvaluestore describe-key-value-store \
    --kvs-arn "$KVS_ARN" --query ETag --output text)"
done

echo "Updated ADMISSION_SECRET and SESSION_SECRET in CloudFront KVS for $ENVIRONMENT"
