package scanner

import (
	"reflect"
	"testing"
)

func TestDetectAWSServices(t *testing.T) {
	services := []*ServiceDef{
		{
			Name: "tasks",
			Functions: []Function{
				{Name: "createTask", Events: []Event{{Type: "http"}, {Type: "sns"}}},
			},
			Resources: []Resource{
				{LogicalID: "TasksTable", Type: "AWS::DynamoDB::Table"},
				{LogicalID: "TaskTopic", Type: "AWS::SNS::Topic"},
			},
		},
		{
			Name: "files",
			Functions: []Function{
				{Name: "upload", Events: []Event{{Type: "httpApi"}}},
			},
			Resources: []Resource{
				{LogicalID: "DocsBucket", Type: "AWS::S3::Bucket"},
			},
		},
	}

	got := DetectAWSServices(services)
	want := []string{"apigateway", "apigatewayv2", "dynamodb", "lambda", "s3", "sns"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("DetectAWSServices() = %v, want %v", got, want)
	}
}

func TestDetectAWSServicesNoFunctionsNoLambda(t *testing.T) {
	services := []*ServiceDef{
		{
			Name:      "infra-only",
			Resources: []Resource{{LogicalID: "Bucket", Type: "AWS::S3::Bucket"}},
		},
	}

	got := DetectAWSServices(services)
	for _, svc := range got {
		if svc == "lambda" {
			t.Errorf("DetectAWSServices() incluyó 'lambda' sin haber funciones: %v", got)
		}
	}
}
