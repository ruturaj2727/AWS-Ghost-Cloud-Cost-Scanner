# aws-ghost

> **Scan your AWS account for forgotten, idle, and wasteful resources — and see exactly what they're costing you.**

Most AWS bills have ghosts in them. Unattached EBS volumes quietly billing at $0.10/GB/month. Elastic IPs reserved but attached to nothing. NAT Gateways in dead VPCs. Snapshots from instances that were terminated a year ago. `aws-ghost` finds all of them in one shot.

```bash
$ aws-ghost scan --region us-east-1

 AWS Ghost Scanner ────────────────────────────────────────────
  Account   123456789012
  Region    us-east-1
  Scanned   21 resource types in 12s

 Ghosts Found: 23 resources — estimated waste: $284.50/month

 EBS Volumes (unattached)                          $109.20/mo
  vol-0a1b2c3d4e   500 GB  gp2   idle 47 days     $50.00/mo
  vol-0f9e8d7c6b   300 GB  gp3   idle 112 days    $36.00/mo
  vol-0123456789   230 GB  gp2   idle 8 days      $23.20/mo

 Elastic IPs (unattached)                          $10.80/mo
  52.14.xxx.xxx    us-east-1a   idle 203 days      $3.60/mo
  18.232.xxx.xxx   us-east-1b   idle 67 days       $3.60/mo
  34.201.xxx.xxx   us-east-1c   idle 31 days       $3.60/mo

 Load Balancers (zero traffic, 7d)                 $48.50/mo
  alb-staging-old   0 req/day   last active 34d    $16.20/mo
  nlb-test-infra    0 req/day   last active 89d    $16.20/mo
  alb-demo-env      0 req/day   last active 14d    $16.10/mo

 NAT Gateways (zero traffic, 7d)                   $97.20/mo
  nat-0a1b2c3d      vpc-legacy   0 bytes/day       $32.40/mo
  nat-0e5f6a7b      vpc-staging  0 bytes/day       $32.40/mo
  nat-0c9d8e7f      vpc-sandbox  0 bytes/day       $32.40/mo

 RDS Snapshots (older than 90 days)                $18.80/mo
  prod-db-snap-2024-01-14    210 GB   311 days old  $6.30/mo
  staging-snap-2023-12-01    180 GB   354 days old  $5.40/mo
  test-db-final-snap         240 GB   290 days old  $7.10/mo

 Estimated annual savings if cleaned: $3,414.00

 Run `aws-ghost report --output markdown > ghost-report.md` to export
──────────────────────────────────────────────────────────────────
```

---

## Why this exists

AWS makes it very easy to create resources and very easy to forget them. `aws-nuke` is a wrecking ball — it deletes everything. `aws-ghost` is the opposite: **read-only, safe, and honest**. It tells you what's wasting money so *you* can decide what to do with it.

| | aws-ghost | aws-nuke | AWS Cost Explorer |
|---|---|---|---|
| Read-only (safe) | ✓ | ✗ | ✓ |
| Per-resource waste estimate | ✓ | ✗ | ✗ |
| Idle detection (not just unattached) | ✓ | ✗ | ✗ |
| CLI / terminal output | ✓ | ✓ | ✗ |
| No SaaS / no account needed | ✓ | ✓ | ✗ |
| Multi-account support | ✓ | ✓ | ✗ |

---

## Install

### Homebrew (macOS / Linux)
```bash
brew install NotHarshhaa/tap/aws-ghost
```

### Go install
```bash
go install github.com/NotHarshhaa/aws-ghost@latest
```

### Binary
```bash
curl -sSL https://github.com/NotHarshhaa/aws-ghost/releases/latest/download/aws-ghost_linux_amd64 \
  -o /usr/local/bin/aws-ghost && chmod +x /usr/local/bin/aws-ghost
```

### Docker
```bash
docker run --rm \
  -e AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY \
  -e AWS_DEFAULT_REGION \
  ghcr.io/NotHarshhaa/aws-ghost scan
```

---

## Usage

`aws-ghost` uses your existing AWS credentials — same as the AWS CLI (`~/.aws/credentials`, env vars, or IAM role).

### Scan everything
```bash
aws-ghost scan
```

### Scan a specific region
```bash
aws-ghost scan --region ap-south-1
```

### Scan all regions at once
```bash
aws-ghost scan --all-regions
```

### Scan specific resource types only
```bash
aws-ghost scan --only ebs,eip,s3,cloudfront
```

### Multi-account via AWS profiles
```bash
aws-ghost scan --profile prod
aws-ghost scan --profile staging
```

### Output formats
```bash
# Default pretty terminal output
aws-ghost scan

# JSON — pipe to jq, feed into scripts
aws-ghost scan --output json | jq '.resources[] | select(.monthly_cost > 20)'

# Markdown — paste into Notion, Confluence, incident doc
aws-ghost scan --output markdown > ghost-report.md

# CSV — open in Excel, share with finance team
aws-ghost scan --output csv > waste-report.csv
```

### Cost threshold filter
```bash
# Only show resources wasting more than $10/month
aws-ghost scan --min-cost 10
```

### Idle threshold
```bash
# Flag resources idle for more than 30 days (default: 7)
aws-ghost scan --idle-days 30
```

### Tag-based filtering
```bash
# Skip resources with protection tags (keep=true, env=prod)
aws-ghost scan --skip-protected

# Only scan resources with specific tag value
aws-ghost scan --tag-filter env=dev

# Group results by tag
aws-ghost scan --group-by owner
```

### Automated cleanup (NEW!)
```bash
# Preview what would be cleaned up (dry run)
aws-ghost fix --dry-run

# Clean up resources under $10/month with confirmation
aws-ghost fix --min-cost 10

# Auto-confirm cleanup for specific resource types
aws-ghost fix --only ebs,eip --auto-confirm

# Force cleanup (skip all confirmations - DANGEROUS)
aws-ghost fix --force --min-cost 5
```

### Cost trends analysis (NEW!)
```bash
# Show trends for the last 30 days
aws-ghost trends --days-back 30

# Export trends to JSON
aws-ghost trends --output trends.json

# Show trends with markdown output
aws-ghost trends --output markdown
```

### Webhook notifications (NEW!)
```bash
# Send scan results to Slack
aws-ghost scan --slack-webhook https://hooks.slack.com/... --notify

# Send trend alerts to Teams
aws-ghost trends --teams-webhook https://outlook.office.com/... --notify

# Test webhook configuration
aws-ghost webhooks --test --slack-webhook https://hooks.slack.com/...
```

### Budget alerts (NEW!)
```bash
# Set a monthly budget of $100 for ghost resource waste
aws-ghost budget --set --amount 100 --period monthly

# Check current waste against budget
aws-ghost budget --check

# Check and send notification if over budget
aws-ghost budget --check --notify --webhook https://hooks.slack.com/...

# View current budget configuration
aws-ghost budget
```

### Cost anomaly detection (NEW!)
```bash
# Detect cost anomalies in the last 30 days
aws-ghost anomaly

# Analyze last 60 days with custom threshold
aws-ghost anomaly --days 60 --threshold 2.5

# Export anomaly results to JSON
aws-ghost anomaly --output json > anomaly-report.json
```

### Resource recommendations (NEW!)
```bash
# Get optimization recommendations
aws-ghost recommend

# Get detailed recommendations with reasoning
aws-ghost recommend --detailed

# Export recommendations to JSON
aws-ghost recommend --output json
```

### Terraform export (NEW!)
```bash
# Generate Terraform destroy plan for ghost resources
aws-ghost terraform

# Export to specific file
aws-ghost terraform --output ghost-destroy.tf

# Include state import commands
aws-ghost terraform --include-state
```

### Scheduled scans (NEW!)
```bash
# Add a scheduled scan (every Monday at 9am)
aws-ghost schedule --cron "0 9 * * 1" --command scan

# List all scheduled scans
aws-ghost schedule --list

# Remove a scheduled scan
aws-ghost schedule --remove sched-123456

# Enable/disable a schedule
aws-ghost schedule --enable sched-123456
aws-ghost schedule --disable sched-123456
```

---

## What it scans

| Resource | Ghost condition | Cost estimate |
|---|---|---|
| EBS Volumes | Unattached for N+ days | $/GB/month by volume type |
| Elastic IPs | Not associated with a running instance | $0.005/hr flat |
| Load Balancers (ALB/NLB/CLB) | Zero requests in last 7 days | LCU + hourly rate |
| NAT Gateways | Zero bytes processed in last 7 days | $0.045/hr + data |
| RDS Snapshots | Older than 90 days, no retention policy | $/GB/month |
| EC2 Snapshots | Older than 90 days, source volume gone | $/GB/month |
| ECR Images | Untagged or last pulled 90+ days ago | $/GB/month |
| Lambda Functions | Zero invocations in last 30 days | negligible, flagged |
| CloudWatch Log Groups | No retention policy set, large size | $/GB ingested |
| Unused Security Groups | Not attached to any resource | informational |
| Empty Target Groups | Registered but no healthy targets | informational |
| Idle EC2 Instances | CPU < 5% avg over 14 days | full instance cost |
| Stopped EC2 Instances | Stopped for 14+ days (EBS still billing) | EBS cost |
| Old AMIs | Not used by any running instance, 90+ days old | snapshot cost |
| S3 Buckets | Empty or contains old objects without lifecycle policy | $/GB/month |
| CloudFront Distributions | Disabled or zero traffic for 30+ days | $0.50/mo + data transfer |
| Auto Scaling Groups | Empty, no instances, or underutilized (<10% CPU) | Instance costs |
| ECS/EKS Clusters | Empty clusters or idle services/node groups | Control plane costs |
| ElastiCache Clusters | Available clusters with low connection/activity | $25/node/month + data |
| OpenSearch Domains | Available domains with minimal search activity | $35/node/month + storage |
| Redshift Clusters | Available clusters with low query activity | $0.25/node/hr + storage |
| DynamoDB Tables | Tables with low read/write activity or old data | $/GB/month + throughput |
| Kinesis Streams | Streams with minimal data throughput | $0.036/shard/hr + retention |
| SQS Queues | Queues with few messages and low activity | $0.40/million requests |
| SNS Topics | Topics with few subscriptions or no publishing | $0.50/million requests |

---

## Flags

| Flag | Description | Default |
|---|---|---|
| `--region` | AWS region to scan | `us-east-1` |
| `--all-regions` | Scan all enabled regions | `false` |
| `--profile` | AWS named profile | default credential chain |
| `--only` | Comma-separated resource types to scan | all |
| `--skip` | Comma-separated resource types to skip | none |
| `--output` | Output format: `text`, `json`, `markdown`, `csv` | `text` |
| `--min-cost` | Only show resources above this monthly cost ($) | `0` |
| `--idle-days` | Days of inactivity to consider a resource idle | `7` |
| `--no-color` | Disable colored terminal output | `false` |
| `--quiet` | Only print the summary line | `false` |

---

## CI/CD integration

Run `aws-ghost` as a scheduled CI job to catch waste before it compounds.

### GitHub Actions
```yaml
name: AWS Ghost Scan
on:
  schedule:
    - cron: '0 9 * * 1'  # Every Monday at 9am

jobs:
  ghost-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install aws-ghost
        run: go install github.com/NotHarshhaa/aws-ghost@latest

      - name: Scan for ghost resources
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          AWS_DEFAULT_REGION: us-east-1
        run: aws-ghost scan --output markdown --min-cost 5 > ghost-report.md

      - name: Upload report
        uses: actions/upload-artifact@v4
        with:
          name: ghost-report
          path: ghost-report.md
```

### GitLab CI
```yaml
aws-ghost-scan:
  image: golang:1.21
  script:
    - go install github.com/NotHarshhaa/aws-ghost@latest
    - aws-ghost scan --all-regions --output markdown > ghost-report.md
  artifacts:
    paths:
      - ghost-report.md
  only:
    - schedules
```

---

## Security & Trust

`aws-ghost` is designed with security and transparency as core principles. Your AWS credentials are never sent to any external service.

### 🔒 Security Guarantees

- **Local-only credential usage**: Your AWS credentials are used locally by the tool and never transmitted to any external servers
- **Read-only operations**: The tool only uses AWS Describe/List API calls — it never creates, modifies, or deletes resources
- **API call transparency**: Every AWS API call made during a scan is logged and displayed in the summary
- **Open source**: The entire codebase is open source and auditable by anyone
- **No account required**: You don't need to create an account or sign up to use this tool

### 🛡️ Security Features

#### Credential Verification
Before each scan, `aws-ghost` displays:
- Your AWS Account ID
- User ARN being used
- Credential source (environment variables, AWS profile, IAM role, etc.)
- Whether root account access is being used (with warning if true)
- MFA status verification

#### Read-Only Verification
The tool verifies that all API operations are read-only:
- Automatic detection of any write operations (Create, Delete, Update, etc.)
- Warning if any non-read operations are detected
- Summary of all services and operations used during the scan

#### Security Audit Command
Run a comprehensive security audit of your credentials:

```bash
aws-ghost security audit
```

This will:
- Verify your credential configuration
- Check for root account usage
- Verify MFA status
- Provide security recommendations
- Display required IAM permissions

#### Security Levels
Configure security strictness with four levels:

```bash
aws-ghost scan --security-level low      # Minimal restrictions (dev/testing)
aws-ghost scan --security-level medium   # Balanced (default)
aws-ghost scan --security-level high     # Enhanced (production)
aws-ghost scan --security-level strict   # Maximum restrictions
```

View available security levels:
```bash
aws-ghost security levels
```

---

## Troubleshooting

### Common Issues

#### Authentication Errors
```
Error: failed to create AWS client: failed to load credentials
```
**Solution**: Ensure your AWS credentials are configured:
- Check `~/.aws/credentials` file exists and has valid credentials
- Or set environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
- Or use `--profile` flag to specify a named profile
- Verify credentials with: `aws sts get-caller-identity`

#### Permission Denied Errors
```
Error: AccessDenied: User is not authorized to perform: ec2:DescribeVolumes
```
**Solution**: Your IAM user/role needs read-only permissions. Attach the policy shown in the "Required IAM permissions" section above.

#### Timeout Errors
```
Error: context deadline exceeded
```
**Solution**: 
- Check your network connectivity to AWS
- Verify security groups allow outbound HTTPS (443) traffic
- Try scanning a single region first: `aws-ghost scan --region us-east-1`
- Increase timeout by setting `AWS_SDK_LOAD_TIMEOUT` environment variable

#### Rate Limiting / Throttling
```
Error: RequestLimitExceeded: Rate exceeded
```
**Solution**: The tool automatically retries with exponential backoff. If you still hit limits:
- Scan fewer resource types: `aws-ghost scan --only ebs,eip`
- Scan one region at a time instead of `--all-regions`
- Wait a few minutes and try again

#### No Resources Found
```
Ghosts Found: 0 resources
```
**Possible reasons**:
- Your account is clean (great!)
- Wrong region: try `--all-regions` or specify correct region
- Resources are tagged with protection tags and you used `--skip-protected`
- `--min-cost` threshold is too high
- `--idle-days` threshold is too high

### Best Practices

#### Regular Scanning
Run `aws-ghost` weekly or monthly to catch waste early:
```bash
# Add to crontab (every Monday at 9am)
0 9 * * 1 aws-ghost scan --all-regions --output markdown > /var/log/ghost-report.md
```

#### Multi-Account Strategy
For organizations with multiple AWS accounts:
```bash
# Scan all accounts using different profiles
for profile in prod staging dev; do
  echo "Scanning $profile..."
  aws-ghost scan --profile $profile --all-regions --output json > "ghost-$profile.json"
done
```

#### Cost Optimization Workflow
1. **Discover**: Run initial scan to identify all ghost resources
2. **Analyze**: Review resources with team, check if any are needed
3. **Tag**: Add `keep=true` tag to resources you want to preserve
4. **Clean**: Use `aws-ghost fix --skip-protected --dry-run` to preview cleanup
5. **Execute**: Run `aws-ghost fix --skip-protected` to delete confirmed ghosts
6. **Monitor**: Set up scheduled scans and budget alerts

#### Tag Strategy
Protect critical resources from accidental cleanup:
```bash
# Tag resources you want to keep
aws ec2 create-tags --resources vol-xxxxx --tags Key=keep,Value=true
aws ec2 create-tags --resources vol-xxxxx --tags Key=env,Value=prod

# Scan with protection
aws-ghost scan --skip-protected
```

#### Integration with CI/CD
Fail builds if waste exceeds threshold:
```bash
#!/bin/bash
WASTE=$(aws-ghost scan --output json | jq '.summary.total_monthly_cost')
if (( $(echo "$WASTE > 100" | bc -l) )); then
  echo "ERROR: Monthly waste ($WASTE) exceeds $100 threshold"
  exit 1
fi
```

#### Security Considerations
- Use read-only IAM roles with minimal permissions
- Enable MFA for production account scans
- Use `--security-level strict` for production environments
- Review audit logs regularly: `aws-ghost scan --audit-log`
- Never use root account credentials

#### Performance Tips
- Scan specific resource types if you know what you're looking for
- Use `--only` flag to limit scope: `aws-ghost scan --only ebs,eip,nat`
- Scan regions in parallel using separate processes
- Cache results and compare trends over time
- Use `--min-cost 5` to filter out negligible waste

### Getting Help

If you encounter issues not covered here:
1. Check existing [GitHub Issues](https://github.com/NotHarshhaa/aws-ghost/issues)
2. Run with verbose output and include in bug report
3. Verify AWS CLI works: `aws ec2 describe-regions`
4. Check tool version: `aws-ghost version`

---

## Required IAM permissions

`aws-ghost` is **read-only**. It will never modify or delete anything.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:Describe*",
        "elasticloadbalancing:Describe*",
        "rds:Describe*",
        "ecr:Describe*",
        "ecr:ListImages",
        "lambda:List*",
        "lambda:GetFunction",
        "logs:Describe*",
        "cloudwatch:GetMetricStatistics",
        "cloudwatch:ListMetrics"
      ],
      "Resource": "*"
    }
  ]
}
```

---

## 🆕 What's New in v2.2.0

### Expanded AWS Service Coverage
- **ElastiCache clusters** detect idle Redis and Memcached clusters with cost estimation
- **OpenSearch domains** identify underutilized OpenSearch domains and their node configurations
- **Redshift clusters** scan for idle data warehouses with accurate cost calculations
- **DynamoDB tables** find tables with low activity or outdated retention policies
- **Kinesis streams** detect idle data streams with shard-based cost analysis
- **SQS queues** identify queues with minimal message activity
- **SNS topics** scan for topics with few or no subscriptions

### Enhanced Cost Analysis
- **Database service optimization** specialized cost detection for managed databases
- **Streaming service efficiency** accurate cost modeling for Kinesis streams
- **Messaging service insights** comprehensive analysis of SQS and SNS usage patterns
- **Cache service monitoring** intelligent detection of ElastiCache waste
- **Search service optimization** OpenSearch domain utilization analysis

### Improved Resource Intelligence
- **Service-specific metrics** tailored idle detection for each AWS service type
- **Enhanced metadata** detailed resource information for better decision making
- **Accurate cost models** updated pricing for all newly supported services
- **Better categorization** organized resource grouping by service family

---

## 🆕 What's New in v2.1.0

### Budget Alerts
- **Set waste budgets** define monthly/weekly/daily spending thresholds
- **Real-time monitoring** check current waste against budget limits
- **Webhook notifications** automatic alerts when budget is exceeded
- **Budget tracking** persistent configuration across scans
- **Multi-period support** flexible budget periods for different use cases

### Cost Anomaly Detection
- **Statistical analysis** detect unusual spending patterns using standard deviation
- **Historical tracking** automatically builds history from scan results
- **Customizable thresholds** adjust sensitivity for anomaly detection
- **Resource-level analysis** identify which resources contribute to anomalies
- **Multi-format export** JSON output for integration with monitoring tools

### Resource Recommendations
- **Prioritized suggestions** high/medium/low priority recommendations
- **Actionable insights** specific steps for cost optimization
- **Resource-specific advice** tailored recommendations per resource type
- **Savings estimation** calculate potential cost impact
- **Best practices** guidance on AWS cost optimization

### Terraform Export
- **Generate destroy plans** export ghost resources as Terraform configuration
- **Controlled cleanup** use Terraform for auditable resource deletion
- **State import commands** optional import commands for existing resources
- **Multi-resource support** handles all supported resource types
- **Safety-first** requires manual review before applying changes

### Scheduled Scans
- **Built-in scheduling** configure automated scans with cron expressions
- **Schedule management** add, list, enable, disable, and remove schedules
- **Multi-schedule support** configure different schedules for different needs
- **Profile/region configuration** per-schedule AWS credentials and regions
- **Manual execution** run scheduled scans on-demand

---

## 🆕 What's New in v2.0

### Automated Cleanup
- **Safe resource deletion** with dry-run mode and confirmations
- **Interactive cleanup** with resource-by-resource approval
- **Tag protection** automatically skips resources with `keep=true`, `env=prod`
- **Force mode** for automated cleanup (with safety checks)

### Cost Trends & Analytics
- **Historical comparison** track waste changes over time
- **Trend analysis** identify increasing/decreasing waste patterns
- **Savings tracking** measure impact of cleanup efforts
- **Multi-format export** JSON, Markdown, and text reports

### Webhook Integrations
- **Slack notifications** rich messages with waste breakdown
- **Teams integration** formatted cards for Microsoft Teams
- **Discord support** embed messages with detailed metrics
- **Trend alerts** automatic notifications for significant changes

### Enhanced Resource Coverage
- **S3 buckets** empty storage and old objects without lifecycle policies
- **CloudFront distributions** disabled or zero-traffic distributions
- **Auto Scaling Groups** empty, underutilized, or misconfigured groups
- **Container services** ECS/EKS clusters and services with no running tasks
- **ElastiCache clusters** idle Redis and Memcached clusters
- **OpenSearch domains** underutilized search domains
- **Redshift clusters** idle data warehouses
- **DynamoDB tables** tables with low activity
- **Kinesis streams** idle data streams
- **SQS queues** inactive message queues
- **SNS topics** unused notification topics

### Tag-Based Filtering
- **Protection tags** skip resources with `keep=true`, `critical=true`
- **Environment filtering** automatic exclusion of `env=prod` resources
- **Resource grouping** organize results by owner, project, or cost center
- **Custom filtering** include/exclude by any tag combination

---

## Roadmap

- [x] `--fix` command — interactive prompt to delete confirmed ghosts one by one
- [x] Slack / Teams webhook: post weekly ghost report automatically
- [x] Tag-based filtering: skip resources tagged `keep=true` or `env=prod`
- [x] Terraform output: generate a `terraform destroy` plan for ghost resources
- [ ] AWS Organizations support: scan all accounts in an org
- [x] Cost trend: compare this week's ghosts vs last week
- [ ] Kubernetes integration: scan for unused PVCs, services, and namespaces
- [x] Cost anomaly detection: alert on unusual spending patterns
- [x] Scheduled scans: built-in cron functionality for automated reporting
- [x] Budget alerts: set and monitor waste budgets
- [x] Resource recommendations: AI-powered optimization suggestions
- [x] ElastiCache support: scan for idle Redis and Memcached clusters
- [x] OpenSearch support: detect underutilized search domains
- [x] Redshift support: identify idle data warehouses
- [x] DynamoDB support: find tables with low activity
- [x] Kinesis support: scan for idle data streams
- [x] SQS support: detect inactive message queues
- [x] SNS support: find unused notification topics

---

## Contributing

```bash
git clone https://github.com/NotHarshhaa/aws-ghost
cd aws-ghost
go mod tidy
go run . scan --region us-east-1
```

Issues and PRs are welcome. If you find a ghost resource type that's not covered, open an issue with the resource type and how to detect it.

---

## License

MIT © [NotHarshhaa](https://github.com/NotHarshhaa)
