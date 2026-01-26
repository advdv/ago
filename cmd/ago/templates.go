package main

import (
	"bytes"
	"os"
	"strings"
	"text/template"

	"github.com/cockroachdb/errors"
)

var accountStackTemplate = template.Must(template.New("account-stack.yaml").Parse(
	`AWSTemplateFormatVersion: '2010-09-09'
Description: Creates an AWS Organizations account for project {{.Qualifier}}

Resources:
  ProjectAccount:
    Type: AWS::Organizations::Account
    Properties:
      AccountName: {{.Qualifier}}
      Email: {{.Email}}
      RoleName: OrganizationAccountAccessRole

Outputs:
  AccountId:
    Description: The AWS Account ID of the created account
    Value: !GetAtt ProjectAccount.AccountId
    Export:
      Name: {{.Qualifier}}-AccountId
  AccountArn:
    Description: The ARN of the created account
    Value: !GetAtt ProjectAccount.Arn
    Export:
      Name: {{.Qualifier}}-AccountArn
`))

var preBootstrapTemplate = template.Must(template.New("pre-bootstrap.cfn.yaml").Parse(
	`AWSTemplateFormatVersion: '2010-09-09'
Transform: AWS::LanguageExtensions
Description: Pre-bootstrap resources for CDK project {{.Qualifier}}

Parameters:
  Qualifier:
    Type: String
    Description: CDK bootstrap qualifier
  SecondaryRegions:
    Type: CommaDelimitedList
    Description: Secondary regions for secret replication
    Default: ""
  Deployers:
    Type: CommaDelimitedList
    Description: List of deployer usernames
    Default: ""
  DevDeployers:
    Type: CommaDelimitedList
    Description: List of dev deployer usernames
    Default: ""

Conditions:
  HasSecondaryRegions: !Not [!Equals [!Join ["", !Ref SecondaryRegions], ""]]
  HasDeployers: !Not [!Equals [!Join ["", !Ref Deployers], ""]]
  HasDevDeployers: !Not [!Equals [!Join ["", !Ref DevDeployers], ""]]

Resources:
  DeployerPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      ManagedPolicyName: !Sub "${Qualifier}-deployer-policy"
      Description: Policy for CDK deployers
      PolicyDocument:
        Version: "2012-10-17"
        Statement:
          - Sid: AssumeCDKRoles
            Effect: Allow
            Action: sts:AssumeRole
            Resource:
              - !Sub "arn:aws:iam::${AWS::AccountId}:role/cdk-${Qualifier}-*"
          - Sid: CloudFormationAccess
            Effect: Allow
            Action:
              - cloudformation:DescribeStacks
              - cloudformation:DescribeStackEvents
              - cloudformation:GetTemplate
              - cloudformation:ListStacks
            Resource: "*"
          - Sid: S3AssetAccess
            Effect: Allow
            Action:
              - s3:GetObject
              - s3:ListBucket
            Resource:
              - !Sub "arn:aws:s3:::cdk-${Qualifier}-assets-${AWS::AccountId}-*"
              - !Sub "arn:aws:s3:::cdk-${Qualifier}-assets-${AWS::AccountId}-*/*"
          - Sid: SSMParameterAccess
            Effect: Allow
            Action:
              - ssm:GetParameter
              - ssm:GetParameters
            Resource: !Sub "arn:aws:ssm:*:${AWS::AccountId}:parameter/cdk-bootstrap/${Qualifier}/*"

  ExecutionPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      ManagedPolicyName: !Sub "${Qualifier}-execution-policy"
      Description: Policy for CDK CloudFormation execution role
      PolicyDocument:
        Version: "2012-10-17"
        Statement:
          - Sid: FullAccess
            Effect: Allow
            Action:
              - apigateway:*
              - cloudfront:*
              - cloudwatch:*
              - cognito-idp:*
              - dynamodb:*
              - ecr:*
              - events:*
              - iam:*
              - lambda:*
              - logs:*
              - route53:*
              - s3:*
              - secretsmanager:*
              - sns:*
              - sqs:*
              - ssm:*
              - states:*
              - acm:*
              - kms:*
            Resource: "*"
          - Sid: CreateServiceLinkedRoles
            Effect: Allow
            Action: iam:CreateServiceLinkedRole
            Resource:
              - !Sub "arn:aws:iam::${AWS::AccountId}:role/aws-service-role/replication.ecr.amazonaws.com/*"
              - !Sub "arn:aws:iam::${AWS::AccountId}:role/aws-service-role/replication.dynamodb.amazonaws.com/*"
              - !Sub "arn:aws:iam::${AWS::AccountId}:role/aws-service-role/ops.apigateway.amazonaws.com/*"
          - Sid: EnforceBoundary
            Effect: Deny
            Action:
              - iam:CreateRole
              - iam:PutRolePermissionsBoundary
            Resource: "*"
            Condition:
              StringNotEquals:
                iam:PermissionsBoundary: !Sub "arn:aws:iam::${AWS::AccountId}:policy/${Qualifier}-permissions-boundary"

  PermissionsBoundary:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      ManagedPolicyName: !Sub "${Qualifier}-permissions-boundary"
      Description: Permission boundary for all CDK-created roles
      PolicyDocument:
        Version: "2012-10-17"
        Statement:
          - Sid: AllowAll
            Effect: Allow
            Action: "*"
            Resource: "*"
          - Sid: DenyBoundaryModification
            Effect: Deny
            Action:
              - iam:DeletePolicy
              - iam:DeletePolicyVersion
              - iam:CreatePolicyVersion
              - iam:SetDefaultPolicyVersion
            Resource: !Sub "arn:aws:iam::${AWS::AccountId}:policy/${Qualifier}-permissions-boundary"
          - Sid: DenyBoundaryRemoval
            Effect: Deny
            Action:
              - iam:DeleteRolePermissionsBoundary
              - iam:DeleteUserPermissionsBoundary
            Resource: "*"
          - Sid: DenyCreateWithoutBoundary
            Effect: Deny
            Action:
              - iam:CreateRole
              - iam:CreateUser
            Resource: "*"
            Condition:
              StringNotEquals:
                iam:PermissionsBoundary: !Sub "arn:aws:iam::${AWS::AccountId}:policy/${Qualifier}-permissions-boundary"

  DeployersGroup:
    Type: AWS::IAM::Group
    Properties:
      GroupName: !Sub "${Qualifier}-deployers"
      ManagedPolicyArns:
        - !Ref DeployerPolicy

  DevDeployersGroup:
    Type: AWS::IAM::Group
    Properties:
      GroupName: !Sub "${Qualifier}-dev-deployers"
      ManagedPolicyArns:
        - !Ref DeployerPolicy

  MainSecret:
    Type: AWS::SecretsManager::Secret
    Properties:
      Name: !Sub "${Qualifier}/main-secret"
      Description: Main project secret
      GenerateSecretString:
        PasswordLength: 32
        ExcludePunctuation: true

  MainSecretReplicaPolicy:
    Type: AWS::SecretsManager::ResourcePolicy
    Condition: HasSecondaryRegions
    Properties:
      SecretId: !Ref MainSecret
      ResourcePolicy:
        Version: "2012-10-17"
        Statement:
          - Sid: AllowReplication
            Effect: Allow
            Principal:
              Service: secretsmanager.amazonaws.com
            Action: secretsmanager:GetSecretValue
            Resource: "*"
            Condition:
              StringEquals:
                aws:SourceAccount: !Ref AWS::AccountId

  GitHubOIDCProvider:
    Type: AWS::IAM::OIDCProvider
    Properties:
      Url: https://token.actions.githubusercontent.com
      ClientIdList:
        - sts.amazonaws.com
      ThumbprintList:
        - 6938fd4d98bab03faadb97b34396831e3780aea1
        - 1c58a3a8518e8759bf075b76b750d4f2df264fcd

  CIDeployerRole:
    Type: AWS::IAM::Role
    Properties:
      RoleName: !Sub "${Qualifier}-ci-deployer"
      AssumeRolePolicyDocument:
        Version: "2012-10-17"
        Statement:
          - Effect: Allow
            Principal:
              Federated: !Ref GitHubOIDCProvider
            Action: sts:AssumeRoleWithWebIdentity
            Condition:
              StringLike:
                token.actions.githubusercontent.com:sub: "repo:*:*"
              StringEquals:
                token.actions.githubusercontent.com:aud: sts.amazonaws.com
      ManagedPolicyArns:
        - !Ref DeployerPolicy

  Fn::ForEach::DeployerUsers:
    - UserName
    - !Ref Deployers
    - ${UserName}DeployerUser:
        Type: AWS::IAM::User
        Condition: HasDeployers
        Properties:
          UserName: ${UserName}
          Groups:
            - !Ref DeployersGroup
      ${UserName}DeployerAccessKey:
        Type: AWS::IAM::AccessKey
        Condition: HasDeployers
        Properties:
          UserName:
            Ref: ${UserName}DeployerUser
      ${UserName}DeployerCredentials:
        Type: AWS::SecretsManager::Secret
        Condition: HasDeployers
        Properties:
          Name: !Sub "${Qualifier}/deployers/${UserName}"
          SecretString:
            Fn::ToJsonString:
              aws_access_key_id:
                Ref: ${UserName}DeployerAccessKey
              aws_secret_access_key:
                Fn::GetAtt:
                  - ${UserName}DeployerAccessKey
                  - SecretAccessKey

  Fn::ForEach::DevDeployerUsers:
    - UserName
    - !Ref DevDeployers
    - ${UserName}DevDeployerUser:
        Type: AWS::IAM::User
        Condition: HasDevDeployers
        Properties:
          UserName: ${UserName}
          Groups:
            - !Ref DevDeployersGroup
      ${UserName}DevDeployerAccessKey:
        Type: AWS::IAM::AccessKey
        Condition: HasDevDeployers
        Properties:
          UserName:
            Ref: ${UserName}DevDeployerUser
      ${UserName}DevDeployerCredentials:
        Type: AWS::SecretsManager::Secret
        Condition: HasDevDeployers
        Properties:
          Name: !Sub "${Qualifier}/deployers/${UserName}"
          SecretString:
            Fn::ToJsonString:
              aws_access_key_id:
                Ref: ${UserName}DevDeployerAccessKey
              aws_secret_access_key:
                Fn::GetAtt:
                  - ${UserName}DevDeployerAccessKey
                  - SecretAccessKey

Outputs:
  ExecutionPolicyArn:
    Description: ARN of the CDK execution policy
    Value: !Ref ExecutionPolicy
    Export:
      Name: !Sub "${Qualifier}-ExecutionPolicyArn"
  PermissionsBoundaryArn:
    Description: ARN of the permissions boundary
    Value: !Ref PermissionsBoundary
    Export:
      Name: !Sub "${Qualifier}-PermissionsBoundaryArn"
  PermissionsBoundaryName:
    Description: Name of the permissions boundary
    Value: !Sub "${Qualifier}-permissions-boundary"
    Export:
      Name: !Sub "${Qualifier}-PermissionsBoundaryName"
  DeployersGroupArn:
    Description: ARN of the deployers group
    Value: !GetAtt DeployersGroup.Arn
    Export:
      Name: !Sub "${Qualifier}-DeployersGroupArn"
  DevDeployersGroupArn:
    Description: ARN of the dev deployers group
    Value: !GetAtt DevDeployersGroup.Arn
    Export:
      Name: !Sub "${Qualifier}-DevDeployersGroupArn"
  CIDeployerRoleArn:
    Description: ARN of the CI deployer role
    Value: !GetAtt CIDeployerRole.Arn
    Export:
      Name: !Sub "${Qualifier}-CIDeployerRoleArn"
`))

type accountStackData struct {
	Qualifier string
	Email     string
}

type preBootstrapData struct {
	Qualifier    string
	Deployers    []string
	DevDeployers []string
}

func renderAccountStackTemplate(qualifier, emailPattern string) (path string, cleanup func(), err error) {
	email := strings.ReplaceAll(emailPattern, "{project}", qualifier)
	data := accountStackData{
		Qualifier: qualifier,
		Email:     email,
	}
	return renderTemplateToTempFile(accountStackTemplate, data, "account-stack-*.yaml")
}

func renderPreBootstrapTemplate(qualifier string) (path string, cleanup func(), err error) {
	data := preBootstrapData{
		Qualifier: qualifier,
	}
	return renderTemplateToTempFile(preBootstrapTemplate, data, "pre-bootstrap-*.yaml")
}

func renderTemplateToTempFile(tmpl *template.Template, data any, pattern string) (string, func(), error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", nil, errors.Wrapf(err, "failed to execute template %s", tmpl.Name())
	}

	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to create temp file")
	}

	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", nil, errors.Wrap(err, "failed to write temp file")
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", nil, errors.Wrap(err, "failed to close temp file")
	}

	cleanup := func() {
		os.Remove(tmpFile.Name())
	}

	return tmpFile.Name(), cleanup, nil
}
