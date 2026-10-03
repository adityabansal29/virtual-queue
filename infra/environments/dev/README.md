# Dev Terraform Environment

Bootstrap the remote state bucket manually in `ap-south-1` before the first
Terraform initialization. This environment uses S3 state only; no DynamoDB
lock table is required.

```bash
aws s3api create-bucket \
  --bucket <your-unique-state-bucket-name> \
  --region ap-south-1 \
  --create-bucket-configuration LocationConstraint=ap-south-1
aws s3api put-bucket-versioning \
  --bucket <your-unique-state-bucket-name> \
  --versioning-configuration Status=Enabled
```

Create `backend.hcl` with the bucket name:

```hcl
bucket = "<your-unique-state-bucket-name>"
```

Set AWS credentials through environment variables or `~/.aws/credentials`,
then run:

```bash
terraform init -backend-config=backend.hcl
terraform plan
terraform apply
```

Application configuration is SSM-owned. Create all six parameters with the
repository script; Terraform only reads them. For example:

```bash
ADMISSION_SECRET="..." \
SESSION_SECRET="..." \
INTERNAL_API_TOKEN="..." \
DEFAULT_ADMIT_RATE=60 \
SSE_THRESHOLD=200 \
SCHEDULER_TICK_SECS=1 \
QUEUE_JOIN_URL="https://<queue-api-cloudfront-domain>/queue/join" \
QUEUE_VALIDATION_URL="https://<queue-api-cloudfront-domain>/admission/validate" \
../../../scripts/set-ssm-config.sh prod
```

Then use the same Terraform root with a separate production state key:

```bash
for key in ADMISSION_SECRET SESSION_SECRET INTERNAL_API_TOKEN; do
  aws ssm put-parameter --name "/virtual-queue/prod/$key" --type SecureString \
    --value "$(openssl rand -hex 32)" --region ap-south-1 --overwrite
done
terraform init -reconfigure -backend-config=backend.hcl \
  -backend-config=key=prod/terraform.tfstate
terraform apply -var-file=prod.tfvars
```

The GitHub workflow handles ECR build/push and ECS service redeploy. Its GitHub
Secrets must contain `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`,
`AWS_REGION`, the three ECR repository URLs, and the ECS cluster/service names.

Terraform uploads `web/queue/index.html`, `queue.js`, and `queue.css` to the
private S3 bucket behind CloudFront. Re-run apply after UI changes, then
invalidate the page distribution if the cached browser page must update
immediately:

```bash
aws cloudfront create-invalidation \
  --distribution-id "$(terraform output -raw queue_page_distribution_id)" \
  --paths '/queue/*'
```

After Terraform creates or replaces the CloudFront KVS, populate its edge
variables from SSM:

```bash
../../../scripts/set-cloudfront-kvs.sh prod
```

The plan should include the networking resources and three ECR repositories.
After the first apply, Terraform outputs the VPC ID, public/private subnet
IDs, and ECR repository URLs.
