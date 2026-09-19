# AgentPay Terraform

Terraform is the only infrastructure-as-code system for AgentPay. The separate
`infra/bootstrap` root creates the remote-state bucket; this root owns the
environment application infrastructure.

## Toolchain

- Terraform `1.16.3`
- HashiCorp AWS provider `6.65.0`

Both versions are pinned. Commit `.terraform.lock.hcl` whenever provider
selection changes.

## Local validation

```powershell
terraform -chdir=infra/terraform fmt -check -recursive
terraform -chdir=infra/terraform init -backend=false
terraform -chdir=infra/terraform validate
```

## Environment setup

Copy, never rename or edit, the committed examples:

```powershell
Copy-Item infra/terraform/environments/dev.backend.hcl.example infra/terraform/environments/dev.backend.hcl
Copy-Item infra/terraform/environments/dev.tfvars.example infra/terraform/environments/dev.tfvars
```

Replace the placeholder account and state-bucket values in the ignored local
copies. Create the encrypted, versioned state bucket through `infra/bootstrap`
before remote initialization succeeds.

```powershell
terraform -chdir=infra/terraform init -backend-config=environments/dev.backend.hcl
terraform -chdir=infra/terraform plan -var-file=environments/dev.tfvars -out=dev.tfplan
```

Do not run `apply` until the plan has been reviewed and the AWS caller identity
matches `aws_account_id`.
