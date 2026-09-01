package scanner

import (
	"sort"
	"strings"
)

// BuildGraph cruza las referencias de cada servicio contra lo que los demás
// exportan/definen para armar las conexiones reales entre ellos.
//
// Tres formas de conexión, de más a menos confiable:
//   - cf-cross-stack: ${cf:stack.Output} — referencia explícita a otro stack
//     de CloudFormation. Se asume que el nombre de stack por defecto de
//     Serverless (`${service}-${stage}`) empieza con el nombre del servicio.
//   - import-value: Fn::ImportValue de un nombre que otro servicio exporta
//     (resources.Outputs.*.Export.Name).
//   - arn-literal: un ARN que menciona un nombre de recurso (cola, tabla,
//     tópico, bucket) que otro servicio declaró literalmente (QueueName,
//     TopicName, TableName, BucketName). Es la más heurística de las tres:
//     dos servicios distintos podrían coincidir en nombre por casualidad.
func BuildGraph(services []*ServiceDef) *Graph {
	exportIndex := make(map[string]string)    // exportName -> serviceName
	resourceIndex := make(map[string]string)  // nombre literal de recurso -> serviceName

	for _, s := range services {
		for _, e := range s.Exports {
			exportIndex[e.ExportName] = s.Name
		}
		for _, r := range s.Resources {
			if r.Name != "" {
				resourceIndex[r.Name] = s.Name
			}
		}
	}

	var edges []Edge
	seen := make(map[string]bool)

	for _, s := range services {
		for _, ref := range s.References {
			target := resolveReference(s, ref, services, exportIndex, resourceIndex)
			if target == "" || target == s.Name {
				continue
			}

			key := s.Name + "\x00" + target + "\x00" + ref.Kind + "\x00" + ref.Raw
			if seen[key] {
				continue
			}
			seen[key] = true

			edges = append(edges, Edge{From: s.Name, To: target, Kind: ref.Kind, Detail: ref.Raw})
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Kind < edges[j].Kind
	})

	return &Graph{Services: services, Edges: edges}
}

func resolveReference(
	self *ServiceDef,
	ref Reference,
	services []*ServiceDef,
	exportIndex, resourceIndex map[string]string,
) string {
	switch ref.Kind {
	case "cf-cross-stack":
		stackPart, outputPart, ok := strings.Cut(ref.Target, ".")
		if !ok {
			return ""
		}
		for _, other := range services {
			if other.Name == self.Name {
				continue
			}
			if strings.HasPrefix(stackPart, other.Name) && hasOutput(other, outputPart) {
				return other.Name
			}
		}
		return ""

	case "import-value":
		return exportIndex[ref.Target]

	case "arn-literal":
		return resourceIndex[ref.Target]

	default:
		return ""
	}
}

func hasOutput(svc *ServiceDef, outputName string) bool {
	for _, e := range svc.Exports {
		if e.OutputName == outputName {
			return true
		}
	}
	return false
}
