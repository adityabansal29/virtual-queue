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

The plan should include the networking resources and three ECR repositories.
After the first apply, Terraform outputs the VPC ID, public/private subnet
IDs, and ECR repository URLs.
