# Security Policy

## Security Guarantees

`aws-ghost` is designed with security as a core principle. The tool operates with the following security guarantees:

### Local-Only Credential Usage
- AWS credentials are used **locally only** and never transmitted to any external servers
- No telemetry, analytics, or data collection is performed
- All AWS API calls are made directly from your machine to AWS endpoints

### Read-Only Operations
- The tool exclusively uses AWS `Describe` and `List` API operations
- No `Create`, `Update`, `Delete`, or other write operations are performed
- Resources are never modified or deleted

### API Call Transparency
- Every AWS API call made during a scan is logged
- A summary of all API calls is displayed at the end of each scan
- Users can verify exactly which operations were performed

### Open Source
- The entire codebase is open source and auditable
- Security researchers can review the code for vulnerabilities
- Community contributions are welcome for security improvements

## Credential Handling

### Credential Sources
`aws-ghost` supports standard AWS credential sources:
- Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`)
- AWS CLI profiles (`~/.aws/credentials` and `~/.aws/config`)
- IAM roles (including EC2 instance roles, ECS task roles, etc.)
- Web identity tokens (for Kubernetes service accounts)

### Credential Verification
Before each scan, the tool displays:
- AWS Account ID
- User ARN being used
- Credential source
- Whether root account access is being used (with warning)
- MFA status verification

### Best Practices
- **Never use root account credentials** for scanning
- Use IAM users with least-privilege permissions
- Enable MFA on IAM users used for scanning
- Rotate access keys regularly
- Use IAM roles where possible (e.g., in EC2, Lambda, ECS)

## Required IAM Permissions

`aws-ghost` requires only read-only permissions:

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

### Minimum Permissions Policy
For production use, consider restricting resources to specific regions or resource tags:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeVolumes",
        "ec2:DescribeAddresses",
        "ec2:DescribeSnapshots",
        "ec2:DescribeInstances",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeNetworkInterfaces"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "elasticloadbalancing:DescribeLoadBalancers",
        "elasticloadbalancing:DescribeTargetGroups"
      ],
      "Resource": "*"
    }
  ]
}
```

## Security Features

### Security Levels
Configure security strictness with four levels:

- **low**: Minimal restrictions for development/testing
- **medium**: Balanced security for general use (default)
- **high**: Enhanced security for production environments
- **strict**: Maximum security with comprehensive restrictions

```bash
aws-ghost scan --security-level high
```

### Security Audit Command
Run a comprehensive security audit:

```bash
aws-ghost security audit
```

This command:
- Verifies credential configuration
- Checks for root account usage
- Verifies MFA status
- Provides security recommendations
- Displays required IAM permissions

### Read-Only Verification
The tool automatically verifies that all API operations are read-only:
- Detects any write operations (Create, Delete, Update, etc.)
- Warns if non-read operations are detected
- Displays summary of all services and operations used

## Vulnerability Reporting

### Reporting a Vulnerability
If you discover a security vulnerability, please report it responsibly:

1. **Do not** create a public issue
2. Send an email to: security@notHarshhaa.com
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if known)

### Response Process
- You will receive a response within 48 hours
- We will work with you to understand and fix the issue
- A fix will be released as soon as possible
- Credit will be given in the release notes (if desired)

### Security Updates
Security updates will be:
- Released as new versions
- Announced in the release notes
- Tagged with security-related labels

## Security Best Practices for Users

### Credential Management
- Use separate IAM users for different tools/environments
- Never commit credentials to version control
- Use environment variables or AWS profiles for credential storage
- Rotate access keys regularly (every 90 days recommended)

### Network Security
- Run `aws-ghost` from trusted networks
- Use VPC endpoints when scanning from within AWS
- Consider using AWS PrivateLink for enhanced security

### Audit and Monitoring
- Enable AWS CloudTrail to monitor API calls made by `aws-ghost`
- Review CloudTrail logs regularly
- Set up alerts for unusual API activity

### CI/CD Integration
- Use OIDC or IAM roles for CI/CD authentication (not long-lived credentials)
- Grant minimum required permissions to CI/CD roles
- Rotate CI/CD credentials regularly

## Known Limitations

### MFA Verification
The tool attempts to verify MFA status but may not always be able to do so reliably without additional IAM permissions. Always verify MFA is enabled on your IAM users independently.

### Credential Age
The tool does not check credential age. Ensure you rotate your access keys regularly according to your organization's security policies.

### Region Restrictions
By default, the tool can scan any AWS region. Use the `--security-level strict` mode or configure allowed regions to restrict scanning to specific regions.

## Compliance

### Data Privacy
- No user data is collected or transmitted
- No personal information is stored
- Scan results are processed locally only

### SOC 2 / ISO 27001
As a CLI tool that runs locally, `aws-ghost` does not store or process data on external servers. Users are responsible for ensuring their use of the tool complies with their organization's security policies.

## Contact

For security-related questions or concerns:
- Email: security@notHarshhaa.com
- GitHub Issues: https://github.com/NotHarshhaa/aws-ghost/issues

## Version History

### Security-Related Changes
- **v1.0.0**: Initial security features including credential verification, read-only verification, API call tracking, and security audit command
