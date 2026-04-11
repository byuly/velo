# Velo — AWS ECS/Fargate Deployment

Minimal-cost deployment for beta (10-20 users).

## Architecture

```
Internet → API Gateway (HTTPS) → VPC Link → Fargate API (0.25 vCPU, 512MB) → RDS Postgres
                                                                                  ↑
           EventBridge (every 5 min) → Fargate Worker (1 vCPU, 2GB) ──────────────┘
                                       (runs, processes reels, exits)
                                             ↓
                                         S3 (clips/reels) → CloudFront CDN
```

**API Gateway HTTP API** — handles TLS and routing. Pay-per-request ($1/million), free tier covers 1M requests/month. Replaces ALB ($16/mo) at essentially $0 for beta traffic.

**API** — always-on Fargate task, serves HTTP. Tiny footprint (no FFmpeg, no scheduler).

**Worker** — triggered every 5 min by EventBridge. Checks for sessions past deadline, generates reels via FFmpeg, then exits. You only pay for the seconds it runs.

No Redis required — uses in-memory token blocklist.

## Prerequisites

- AWS CLI configured
- Docker installed locally
- A domain name (for TLS via ACM)

## Setup Steps

### 1. Create ECR Repository

```bash
aws ecr create-repository --repository-name velo-api --region us-west-2
```

### 2. Build and Push Image

```bash
aws ecr get-login-password --region us-west-2 | \
  docker login --username AWS --password-stdin ACCOUNT_ID.dkr.ecr.us-west-2.amazonaws.com

cd server
VERSION=$(git describe --tags --always)
docker build --build-arg VERSION=$VERSION -t velo-api .
docker tag velo-api:latest ACCOUNT_ID.dkr.ecr.us-west-2.amazonaws.com/velo-api:latest
docker push ACCOUNT_ID.dkr.ecr.us-west-2.amazonaws.com/velo-api:latest
```

Both API and worker binaries are in the same image. The `command` field in each task definition selects which binary runs.

### 3. Create RDS Postgres Instance

```bash
aws rds create-db-instance \
  --db-instance-identifier velo-db \
  --db-instance-class db.t4g.micro \
  --engine postgres \
  --engine-version 16 \
  --master-username velo \
  --master-user-password <SECURE_PASSWORD> \
  --allocated-storage 20 \
  --db-name velo \
  --vpc-security-group-ids <SG_ID> \
  --no-publicly-accessible
```

### 4. Store Secrets in SSM Parameter Store

```bash
aws ssm put-parameter --name /velo/DATABASE_URL \
  --type SecureString \
  --value "postgres://velo:<PASSWORD>@<RDS_ENDPOINT>:5432/velo?sslmode=require"

aws ssm put-parameter --name /velo/JWT_SECRET \
  --type SecureString \
  --value "<MIN_32_BYTE_SECRET>"
```

### 5. Create IAM Roles

**Execution Role** (pulls images, reads SSM):
- Attach `AmazonECSTaskExecutionRolePolicy`
- Add inline policy for SSM `GetParameter` on `/velo/*`

**Task Role** (runtime S3 access):
- Allow `s3:GetObject`, `s3:PutObject`, `s3:HeadObject` on `velo-clips/*` and `velo-reels/*`

**Events Role** (EventBridge → ECS):
- Allow `ecs:RunTask` and `iam:PassRole`

With the task role, `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` can be omitted.

### 6. Create ECS Cluster + Cloud Map Namespace

```bash
# ECS cluster
aws ecs create-cluster --cluster-name velo --region us-west-2

# Cloud Map namespace for service discovery (API Gateway needs this)
aws servicediscovery create-private-dns-namespace \
  --name velo.local \
  --vpc VPC_ID \
  --region us-west-2
```

Save the namespace ID from the output — you'll need it in step 9.

### 7. Register Task Definitions

Edit both JSON files — replace `ACCOUNT_ID` and `REGION`:

```bash
aws ecs register-task-definition --cli-input-json file://deploy/ecs-task-definition.json
aws ecs register-task-definition --cli-input-json file://deploy/ecs-worker-task-definition.json
```

### 8. Create API Gateway HTTP API + VPC Link

```bash
# Create VPC Link (connects API Gateway to your private VPC)
aws apigatewayv2 create-vpc-link \
  --name velo-vpc-link \
  --subnet-ids SUBNET_ID_1 SUBNET_ID_2 \
  --security-group-ids SG_ID

# Create the HTTP API
aws apigatewayv2 create-api \
  --name velo-api \
  --protocol-type HTTP

# Create the integration (routes to Cloud Map service via VPC Link)
aws apigatewayv2 create-integration \
  --api-id API_ID \
  --integration-type HTTP_PROXY \
  --integration-method ANY \
  --integration-uri "arn:aws:servicediscovery:REGION:ACCOUNT_ID:service/SERVICE_ID" \
  --connection-type VPC_LINK \
  --connection-id VPC_LINK_ID \
  --payload-format-version 1.0

# Create catch-all route
aws apigatewayv2 create-route \
  --api-id API_ID \
  --route-key '$default' \
  --target "integrations/INTEGRATION_ID"

# Deploy
aws apigatewayv2 create-stage \
  --api-id API_ID \
  --stage-name '$default' \
  --auto-deploy
```

### 9. Create API Service (Always-On) with Service Discovery

```bash
# Create Cloud Map service
aws servicediscovery create-service \
  --name api \
  --namespace-id NAMESPACE_ID \
  --dns-config "NamespaceId=NAMESPACE_ID,DnsRecords=[{Type=SRV,TTL=60}]" \
  --health-check-custom-config FailureThreshold=1

# Create ECS service with service discovery
aws ecs create-service \
  --cluster velo \
  --service-name velo-api \
  --task-definition velo-api \
  --desired-count 1 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={subnets=[SUBNET_IDS],securityGroups=[SG_ID],assignPublicIp=DISABLED}" \
  --service-registries "registryArn=arn:aws:servicediscovery:REGION:ACCOUNT_ID:service/SERVICE_ID,containerName=api,containerPort=8080"
```

Note: `assignPublicIp=DISABLED` — the API sits behind API Gateway via VPC Link, no public IP needed.

### 10. Custom Domain + TLS

```bash
# Request ACM certificate (must be in the same region as API Gateway)
aws acm request-certificate \
  --domain-name api.velo.app \
  --validation-method DNS

# Add custom domain to API Gateway
aws apigatewayv2 create-domain-name \
  --domain-name api.velo.app \
  --domain-name-configurations "CertificateArn=CERT_ARN"

# Map domain to the API
aws apigatewayv2 create-api-mapping \
  --api-id API_ID \
  --domain-name api.velo.app \
  --stage '$default'
```

Point `api.velo.app` to the API Gateway domain name via Route 53 (A record alias) or CNAME.

### 11. Create EventBridge Scheduled Rule (Worker)

The worker runs every 5 minutes, processes any due sessions, then exits:

```bash
# Create the schedule rule
aws events put-rule \
  --name velo-reel-worker \
  --schedule-expression "rate(5 minutes)" \
  --state ENABLED

# Create the ECS target
aws events put-targets \
  --rule velo-reel-worker \
  --targets '[{
    "Id": "velo-worker",
    "Arn": "arn:aws:ecs:REGION:ACCOUNT_ID:cluster/velo",
    "RoleArn": "arn:aws:iam::ACCOUNT_ID:role/ecsEventsRole",
    "EcsParameters": {
      "TaskDefinitionArn": "arn:aws:ecs:REGION:ACCOUNT_ID:task-definition/velo-worker",
      "TaskCount": 1,
      "LaunchType": "FARGATE",
      "NetworkConfiguration": {
        "awsvpcConfiguration": {
          "Subnets": ["SUBNET_ID"],
          "SecurityGroups": ["SG_ID"],
          "AssignPublicIp": "ENABLED"
        }
      }
    }
  }]'
```

## Deploying Updates

```bash
cd server
VERSION=$(git describe --tags --always)
docker build --build-arg VERSION=$VERSION -t velo-api .
docker tag velo-api:latest ACCOUNT_ID.dkr.ecr.us-west-2.amazonaws.com/velo-api:latest
docker push ACCOUNT_ID.dkr.ecr.us-west-2.amazonaws.com/velo-api:latest

# Restart API (picks up new image)
aws ecs update-service --cluster velo --service velo-api --force-new-deployment
```

Worker automatically uses the latest image on next EventBridge trigger.
Migrations run automatically on both API and worker startup.

## Estimated Monthly Cost (10-20 beta users)

| Component | Spec | Cost |
|-----------|------|------|
| Fargate API | 0.25 vCPU, 512MB, always-on | ~$8 |
| Fargate Worker | 1 vCPU, 2GB, ~5 min/day actual runtime | ~$0.15 |
| RDS Postgres | db.t4g.micro, 20GB | ~$13 |
| API Gateway | HTTP API, pay-per-request | ~$0 (free tier) |
| S3 + CloudFront | minimal usage | < $2 |
| EventBridge | 8,640 invocations/mo | free tier |
| CloudWatch Logs | minimal | < $1 |
| **Total** | | **~$24/mo** |

## Adding Redis (Optional)

If you need Redis (persistent token blocklist across restarts):

1. Create ElastiCache Redis (cache.t4g.micro, ~$12/mo)
2. Add `REDIS_ADDR` to the API task definition
3. Update security group for port 6379

Without Redis, revoked tokens are lost on API container restart. With 60-min JWT TTL, this is acceptable for beta.

## Scaling Beyond Beta

When you outgrow this setup:
- **More users**: Increase API desired count to 2+ (API Gateway handles routing automatically)
- **More reels**: Increase worker CPU/memory, or run multiple concurrent workers (the `FOR UPDATE SKIP LOCKED` query prevents double-processing)
- **Lower latency**: Decrease EventBridge interval to 1 minute
- **Always-on worker**: Switch to an ECS service with the scheduler's `Run()` loop instead of `RunOnce()`
- **Switch to ALB**: If you hit API Gateway's 30s request timeout or need WebSocket support, swap to ALB (~$16/mo more)
