package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/openshift-online/rosa-hyperfleet-zoa/internal/output"
)

func TestParseSSMDeploymentParameters_WhenValidJSON_ItShouldDecodeDeployments(t *testing.T) {
	params := []ssmtypes.Parameter{
		{
			Name: aws.String("/zoa/deployments/us-east-1"),
			Value: aws.String(`{
				"deployment_name": "us-east-1",
				"access_url": "https://example.lambda-url.us-east-1.on.aws",
				"invoker_role_arn": "arn:aws:iam::123:role/invoker",
				"region": "us-east-1",
				"account_id": "123"
			}`),
		},
	}

	deployments, err := parseSSMDeploymentParameters(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deployments) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(deployments))
	}
	if deployments[0].DeploymentName != "us-east-1" {
		t.Errorf("expected deployment name us-east-1, got %q", deployments[0].DeploymentName)
	}
	if deployments[0].AccessURL == "" {
		t.Error("expected access_url to be set")
	}
}

func TestParseSSMDeploymentParameters_WhenInvalidJSON_ItShouldReturnError(t *testing.T) {
	params := []ssmtypes.Parameter{
		{
			Name:  aws.String("/zoa/deployments/bad"),
			Value: aws.String("{not-json"),
		},
	}

	_, err := parseSSMDeploymentParameters(params)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestPrintDeploymentsTable_WhenDefault_ItShouldOmitSensitiveColumns(t *testing.T) {
	deployments := []ssmDeployment{{
		DeploymentName: "us-east-1-eph",
		Region:         "us-east-1",
		AccessURL:      "https://secret.lambda-url.on.aws/",
		InvokerRoleARN: "arn:aws:iam::123:role/invoker",
		AccountID:      "123456789012",
	}}

	var buf bytes.Buffer
	opts := &GlobalOptions{OutputFormat: output.FormatTable}
	if err := printDeploymentsTable(&buf, opts, deployments); err != nil {
		t.Fatalf("printDeploymentsTable: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "ACCESS URL") || strings.Contains(out, "lambda-url") {
		t.Fatalf("default table should not show URLs, got:\n%s", out)
	}
	if !strings.Contains(out, "us-east-1-eph") {
		t.Fatalf("expected deployment name in output, got:\n%s", out)
	}
}

func TestPrintDeploymentsTable_WhenWide_ItShouldIncludeAllMetadata(t *testing.T) {
	url := "https://secret.lambda-url.on.aws/"
	role := "arn:aws:iam::123:role/invoker"
	deployments := []ssmDeployment{{
		DeploymentName: "us-east-1-eph",
		Region:         "us-east-1",
		AccessURL:      url,
		InvokerRoleARN: role,
		AccountID:      "123456789012",
	}}

	var buf bytes.Buffer
	opts := &GlobalOptions{OutputFormat: output.FormatWide}
	if err := printDeploymentsTable(&buf, opts, deployments); err != nil {
		t.Fatalf("printDeploymentsTable: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"ACCESS URL", "INVOKER ROLE", "ACCOUNT ID", url, role, "123456789012"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected wide output to contain %q, got:\n%s", want, out)
		}
	}
}
