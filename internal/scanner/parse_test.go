package scanner

import (
	"path/filepath"
	"testing"
)

const ordersYAML = `
service: orders

provider:
  name: aws
  runtime: nodejs20.x

functions:
  createOrder:
    handler: handler.create
    events:
      - http:
          path: /orders
          method: post

resources:
  Resources:
    OrdersQueue:
      Type: AWS::SQS::Queue
      Properties:
        QueueName: orders-queue
  Outputs:
    OrdersQueueArn:
      Value:
        Fn::GetAtt: [OrdersQueue, Arn]
      Export:
        Name: orders-dev-OrdersQueueArn
`

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "serverless.yml")
	writeFile(t, path, ordersYAML)

	svc, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if svc.Name != "orders" {
		t.Errorf("Name = %q, want %q", svc.Name, "orders")
	}

	if len(svc.Functions) != 1 {
		t.Fatalf("len(Functions) = %d, want 1", len(svc.Functions))
	}
	fn := svc.Functions[0]
	if fn.Name != "createOrder" || fn.Handler != "handler.create" {
		t.Errorf("Functions[0] = %+v, want name=createOrder handler=handler.create", fn)
	}
	if len(fn.Events) != 1 || fn.Events[0].Type != "http" || fn.Events[0].Detail != "/orders" {
		t.Errorf("Functions[0].Events = %+v, want un http event con detail /orders", fn.Events)
	}

	if len(svc.Resources) != 1 {
		t.Fatalf("len(Resources) = %d, want 1", len(svc.Resources))
	}
	res := svc.Resources[0]
	if res.LogicalID != "OrdersQueue" || res.Type != "AWS::SQS::Queue" || res.Name != "orders-queue" {
		t.Errorf("Resources[0] = %+v, want LogicalID=OrdersQueue Type=AWS::SQS::Queue Name=orders-queue", res)
	}

	if len(svc.Exports) != 1 {
		t.Fatalf("len(Exports) = %d, want 1", len(svc.Exports))
	}
	exp := svc.Exports[0]
	if exp.OutputName != "OrdersQueueArn" || exp.ExportName != "orders-dev-OrdersQueueArn" {
		t.Errorf("Exports[0] = %+v, want OutputName=OrdersQueueArn ExportName=orders-dev-OrdersQueueArn", exp)
	}
}

func TestParseFileServiceAsMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "serverless.yml")
	writeFile(t, path, "service:\n  name: mi-servicio\nfunctions: {}\n")

	svc, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if svc.Name != "mi-servicio" {
		t.Errorf("Name = %q, want %q (sintaxis 'service: { name: ... }')", svc.Name, "mi-servicio")
	}
}
