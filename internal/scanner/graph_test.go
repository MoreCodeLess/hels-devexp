package scanner

import (
	"path/filepath"
	"testing"
)

const shippingYAML = `
service: shipping

functions:
  handleOrderByArn:
    handler: handler.handleByArn
    events:
      - sqs:
          arn: arn:aws:sqs:us-east-1:000000000000:orders-queue
  handleOrderByStack:
    handler: handler.handleByStack
    events:
      - sqs:
          arn: ${cf:orders-dev.OrdersQueueArn}
`

const notificationsYAML = `
service: notifications

functions:
  notify:
    handler: handler.notify
    events:
      - schedule: rate(5 minutes)

resources:
  Resources:
    Dummy:
      Type: AWS::SNS::Topic
`

func TestBuildGraphDetectsAllReferenceKinds(t *testing.T) {
	dir := t.TempDir()

	// notifications hace Fn::ImportValue del export de orders. Se escribe
	// como YAML crudo (no via el struct Function) porque environment/
	// Fn::ImportValue no tienen un campo dedicado en el parser — a
	// findReferences no le importa dónde aparece el Fn::ImportValue, solo
	// que aparezca en el documento.
	notificationsWithImport := notificationsYAML + `
    environment:
      QUEUE_ARN:
        Fn::ImportValue: orders-dev-OrdersQueueArn
`

	ordersPath := filepath.Join(dir, "orders", "serverless.yml")
	shippingPath := filepath.Join(dir, "shipping", "serverless.yml")
	notificationsPath := filepath.Join(dir, "notifications", "serverless.yml")

	writeFile(t, ordersPath, ordersYAML)
	writeFile(t, shippingPath, shippingYAML)
	writeFile(t, notificationsPath, notificationsWithImport)

	var services []*ServiceDef
	for _, path := range []string{ordersPath, shippingPath, notificationsPath} {
		svc, err := ParseFile(path)
		if err != nil {
			t.Fatalf("ParseFile(%s) error = %v", path, err)
		}
		services = append(services, svc)
	}

	g := BuildGraph(services)

	want := map[string]bool{
		"shipping\x00orders\x00arn-literal":     false,
		"shipping\x00orders\x00cf-cross-stack":  false,
		"notifications\x00orders\x00import-value": false,
	}
	for _, e := range g.Edges {
		key := e.From + "\x00" + e.To + "\x00" + e.Kind
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Errorf("no se encontró la conexión esperada %q entre las %d detectadas: %+v", key, len(g.Edges), g.Edges)
		}
	}
}

const selfReferencingYAML = `
service: orders

functions:
  consumeOwnQueue:
    handler: handler.consume
    events:
      - sqs:
          arn: arn:aws:sqs:us-east-1:000000000000:orders-queue

resources:
  Resources:
    OrdersQueue:
      Type: AWS::SQS::Queue
      Properties:
        QueueName: orders-queue
`

func TestBuildGraphNoFalseSelfEdges(t *testing.T) {
	// Un servicio que se referencia a sí mismo (ej. su propia cola en un ARN
	// literal) no debería generar una conexión.
	dir := t.TempDir()
	path := filepath.Join(dir, "serverless.yml")
	writeFile(t, path, selfReferencingYAML)

	svc, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	g := BuildGraph([]*ServiceDef{svc})
	for _, e := range g.Edges {
		if e.From == e.To {
			t.Errorf("BuildGraph() generó una auto-conexión: %+v", e)
		}
	}
}
