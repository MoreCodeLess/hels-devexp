package scanner

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

var literalNameProps = []string{"QueueName", "TopicName", "TableName", "BucketName"}

// ParseFile lee y parsea un serverless.yml/yaml en un ServiceDef.
func ParseFile(path string) (*ServiceDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", path, err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parseando %s: %w", path, err)
	}

	svc := &ServiceDef{
		Name:    serviceName(doc, path),
		Path:    path,
		Runtime: providerRuntime(doc),
	}

	svc.Functions = parseFunctions(doc)
	svc.Resources = parseResources(doc)
	svc.Exports = parseExports(doc)
	svc.References = findReferences(doc)

	return svc, nil
}

func providerRuntime(doc map[string]interface{}) string {
	provider, ok := doc["provider"].(map[string]interface{})
	if !ok {
		return ""
	}
	runtime, _ := provider["runtime"].(string)
	return runtime
}

func serviceName(doc map[string]interface{}, path string) string {
	switch v := doc["service"].(type) {
	case string:
		return v
	case map[string]interface{}:
		if name, ok := v["name"].(string); ok {
			return name
		}
	}
	return path
}

func parseFunctions(doc map[string]interface{}) []Function {
	raw, ok := doc["functions"].(map[string]interface{})
	if !ok {
		return nil
	}

	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names)

	functions := make([]Function, 0, len(names))
	for _, name := range names {
		def, ok := raw[name].(map[string]interface{})
		if !ok {
			functions = append(functions, Function{Name: name})
			continue
		}

		fn := Function{Name: name}
		if handler, ok := def["handler"].(string); ok {
			fn.Handler = handler
		}
		fn.Events = parseEvents(def["events"])
		functions = append(functions, fn)
	}
	return functions
}

func parseEvents(raw interface{}) []Event {
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	var events []Event
	for _, item := range list {
		def, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		for evType, evDef := range def {
			events = append(events, Event{
				Type:   evType,
				Detail: eventDetail(evDef),
			})
		}
	}
	return events
}

// eventDetail arma una descripción corta y legible de un event, para
// mostrar en la salida en texto (ej. el path de un http event, el nombre de
// una cola de un sqs event).
func eventDetail(evDef interface{}) string {
	switch v := evDef.(type) {
	case string:
		return v
	case map[string]interface{}:
		for _, key := range []string{"path", "arn", "queue", "topic", "schedule", "bucket"} {
			if s, ok := v[key].(string); ok {
				return s
			}
		}
	}
	return ""
}

func parseResources(doc map[string]interface{}) []Resource {
	resourcesBlock, ok := doc["resources"].(map[string]interface{})
	if !ok {
		return nil
	}
	rawResources, ok := resourcesBlock["Resources"].(map[string]interface{})
	if !ok {
		return nil
	}

	logicalIDs := make([]string, 0, len(rawResources))
	for id := range rawResources {
		logicalIDs = append(logicalIDs, id)
	}
	sort.Strings(logicalIDs)

	resources := make([]Resource, 0, len(logicalIDs))
	for _, id := range logicalIDs {
		def, ok := rawResources[id].(map[string]interface{})
		if !ok {
			resources = append(resources, Resource{LogicalID: id})
			continue
		}

		res := Resource{LogicalID: id}
		if t, ok := def["Type"].(string); ok {
			res.Type = t
		}
		if props, ok := def["Properties"].(map[string]interface{}); ok {
			res.Properties = props
			for _, prop := range literalNameProps {
				if name, ok := props[prop].(string); ok {
					res.Name = name
					break
				}
			}
		}
		resources = append(resources, res)
	}
	return resources
}

func parseExports(doc map[string]interface{}) []Export {
	resourcesBlock, ok := doc["resources"].(map[string]interface{})
	if !ok {
		return nil
	}
	rawOutputs, ok := resourcesBlock["Outputs"].(map[string]interface{})
	if !ok {
		return nil
	}

	names := make([]string, 0, len(rawOutputs))
	for name := range rawOutputs {
		names = append(names, name)
	}
	sort.Strings(names)

	var exports []Export
	for _, name := range names {
		def, ok := rawOutputs[name].(map[string]interface{})
		if !ok {
			continue
		}
		exportBlock, ok := def["Export"].(map[string]interface{})
		if !ok {
			continue
		}
		exportName, ok := exportBlock["Name"].(string)
		if !ok {
			continue
		}
		exports = append(exports, Export{OutputName: name, ExportName: exportName})
	}
	return exports
}
