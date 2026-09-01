package scanner

import (
	"regexp"
	"strings"
)

// cfPattern matea variables ${cf:stack-name.OutputName}: la forma en que
// Serverless Framework referencia el output de OTRO stack de CloudFormation.
// Es la señal más confiable de conexión entre dos servicios, porque es una
// referencia explícita, no una coincidencia heurística de nombres.
var cfPattern = regexp.MustCompile(`\$\{cf:([a-zA-Z0-9_.\-]+)\.([a-zA-Z0-9_.\-]+)\}`)

// arnPattern matea ARNs de AWS: arn:aws:<servicio>:<region>:<cuenta>:<resto>.
var arnPattern = regexp.MustCompile(`^arn:aws:([a-zA-Z0-9\-]+):[^:]*:[^:]*:(.+)$`)

// findReferences recorre recursivamente un valor ya parseado de YAML
// (map[string]interface{} / []interface{} / escalares, como lo devuelve
// yaml.Unmarshal a interface{}) buscando referencias a recursos externos.
func findReferences(node interface{}) []Reference {
	var refs []Reference
	walkNode(node, &refs)
	return refs
}

func walkNode(node interface{}, refs *[]Reference) {
	switch v := node.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if key == "Fn::ImportValue" {
				if s, ok := val.(string); ok {
					*refs = append(*refs, Reference{
						Kind:   "import-value",
						Target: s,
						Raw:    s,
					})
					continue
				}
			}
			walkNode(val, refs)
		}
	case []interface{}:
		for _, item := range v {
			walkNode(item, refs)
		}
	case string:
		refs = appendStringReferences(v, refs)
	}
}

func appendStringReferences(s string, refs *[]Reference) *[]Reference {
	for _, m := range cfPattern.FindAllStringSubmatch(s, -1) {
		*refs = append(*refs, Reference{
			Kind:   "cf-cross-stack",
			Target: m[1] + "." + m[2],
			Raw:    m[0],
		})
	}

	if m := arnPattern.FindStringSubmatch(s); m != nil {
		name := m[2]
		if idx := strings.LastIndex(name, "/"); idx != -1 {
			name = name[idx+1:]
		}
		*refs = append(*refs, Reference{
			Kind:   "arn-literal",
			Target: name,
			Raw:    s,
		})
	}

	return refs
}
