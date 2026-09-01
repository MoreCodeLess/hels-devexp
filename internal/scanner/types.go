// Package scanner descubre archivos serverless.yml/yaml bajo una ruta,
// extrae los servicios que definen (funciones, recursos, triggers) y arma un
// grafo de cómo se conectan entre sí (vía referencias cruzadas de
// CloudFormation, ARNs y nombres de recursos compartidos).
package scanner

// Event es un trigger de una función (http, sqs, sns, schedule, stream, ...).
type Event struct {
	Type   string `json:"type"`
	Detail string `json:"detail,omitempty"`
}

// Function es una función Lambda declarada en el serverless.yml.
type Function struct {
	Name    string  `json:"name"`
	Handler string  `json:"handler,omitempty"`
	Events  []Event `json:"events,omitempty"`
}

// Resource es un recurso de CloudFormation declarado en resources.Resources.
type Resource struct {
	LogicalID string `json:"logicalId"`
	Type      string `json:"type,omitempty"`
	// Name es el nombre literal del recurso si se pudo extraer de sus
	// Properties (QueueName, TopicName, TableName, BucketName, ...).
	Name string `json:"name,omitempty"`
	// Properties son las Properties crudas del recurso, tal como las parseó
	// yaml.Unmarshal. internal/deploy las usa para crear el recurso de verdad
	// (ej. AttributeDefinitions/KeySchema de una tabla DynamoDB) sin que este
	// paquete tenga que modelar el schema completo de cada tipo de recurso.
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// Export es un output de CloudFormation exportado (resources.Outputs.*.Export.Name),
// lo que otros stacks pueden importar con Fn::ImportValue.
type Export struct {
	OutputName string `json:"outputName"`
	ExportName string `json:"exportName"`
}

// Reference es algo que un servicio parece consumir de afuera: una
// referencia cruzada a otro stack, un Fn::ImportValue, o un ARN/nombre
// literal que podría apuntar a un recurso de otro servicio.
type Reference struct {
	Kind string `json:"kind"` // "cf-cross-stack" | "import-value" | "arn-literal"
	// Target es la clave de búsqueda: "stack.output" para cf-cross-stack,
	// el nombre exportado para import-value, o el nombre de recurso extraído
	// del ARN para arn-literal.
	Target string `json:"target"`
	// Raw es el string original, para mostrarle al usuario de dónde salió.
	Raw string `json:"raw"`
}

// ServiceDef es todo lo que se pudo extraer de un serverless.yml.
type ServiceDef struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Runtime es provider.runtime (ej. "nodejs20.x"). internal/deploy lo usa
	// para saber cómo empaquetar y correr las funciones.
	Runtime    string      `json:"runtime,omitempty"`
	Functions  []Function  `json:"functions,omitempty"`
	Resources  []Resource  `json:"resources,omitempty"`
	Exports    []Export    `json:"exports,omitempty"`
	References []Reference `json:"references,omitempty"`
}

// Edge es una conexión detectada entre dos servicios.
type Edge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// Graph es el resultado completo de escanear una ruta.
type Graph struct {
	Services []*ServiceDef `json:"services"`
	Edges    []Edge        `json:"edges"`
}
