package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// RenderText imprime un resumen legible del grafo: cada servicio con sus
// funciones, y las conexiones detectadas entre servicios.
func RenderText(w io.Writer, g *Graph) {
	names := make([]string, 0, len(g.Services))
	byName := make(map[string]*ServiceDef, len(g.Services))
	for _, s := range g.Services {
		names = append(names, s.Name)
		byName[s.Name] = s
	}
	sort.Strings(names)

	for _, name := range names {
		s := byName[name]
		fmt.Fprintf(w, "%s (%s)\n", s.Name, s.Path)
		for _, fn := range s.Functions {
			fmt.Fprintf(w, "  - %s", fn.Name)
			if fn.Handler != "" {
				fmt.Fprintf(w, " (%s)", fn.Handler)
			}
			fmt.Fprintln(w)
			for _, ev := range fn.Events {
				if ev.Detail != "" {
					fmt.Fprintf(w, "      %s: %s\n", ev.Type, ev.Detail)
				} else {
					fmt.Fprintf(w, "      %s\n", ev.Type)
				}
			}
		}
		if len(s.Functions) == 0 {
			fmt.Fprintln(w, "  (sin funciones)")
		}
	}

	fmt.Fprintln(w)
	if len(g.Edges) == 0 {
		fmt.Fprintln(w, "Conexiones detectadas: ninguna")
		return
	}
	fmt.Fprintln(w, "Conexiones detectadas:")
	for _, e := range g.Edges {
		fmt.Fprintf(w, "  %s -> %s  [%s: %s]\n", e.From, e.To, e.Kind, e.Detail)
	}
}

// RenderJSON escribe el grafo completo como JSON.
func RenderJSON(w io.Writer, g *Graph) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(g)
}

// RenderMermaid arma un diagrama de flujo Mermaid ("graph LR") con un nodo
// por servicio y una flecha por conexión detectada.
func RenderMermaid(w io.Writer, g *Graph) {
	fmt.Fprintln(w, "graph LR")
	for _, s := range g.Services {
		fmt.Fprintf(w, "    %s[%s]\n", mermaidID(s.Name), s.Name)
	}
	for _, e := range g.Edges {
		fmt.Fprintf(w, "    %s -->|%s| %s\n", mermaidID(e.From), e.Kind, mermaidID(e.To))
	}
}

// mermaidID sanitiza un nombre de servicio para usarlo como ID de nodo en
// Mermaid (sin espacios ni guiones, que rompen la sintaxis).
func mermaidID(name string) string {
	replacer := strings.NewReplacer("-", "_", " ", "_", ".", "_")
	return replacer.Replace(name)
}
