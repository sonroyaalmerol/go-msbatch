package analyzer

import (
	"path/filepath"
	"slices"
	"strings"
)

type WorkspaceResult struct {
	Documents map[string]*Result
	CallGraph map[string][]string
}

type WorkspaceAnalyzer struct {
	analyzer  *Analyzer
	documents map[string]*Result
	callGraph map[string][]string
}

func NewWorkspaceAnalyzer() *WorkspaceAnalyzer {
	return &WorkspaceAnalyzer{
		analyzer:  NewAnalyzer(),
		documents: make(map[string]*Result),
		callGraph: make(map[string][]string),
	}
}

func (wa *WorkspaceAnalyzer) AnalyzeWorkspace(files map[string]string) *WorkspaceResult {
	for uri, content := range files {
		result := wa.analyzer.Analyze(uri, content)
		wa.documents[uri] = result
	}

	wa.buildCallGraph()

	wa.propagateVariables()

	return &WorkspaceResult{
		Documents: wa.documents,
		CallGraph: wa.callGraph,
	}
}

func (wa *WorkspaceAnalyzer) AnalyzeDocument(uri, content string) *Result {
	result := wa.analyzer.Analyze(uri, content)
	wa.documents[uri] = result

	wa.rebuildCallGraph()

	return result
}

func (wa *WorkspaceAnalyzer) rebuildCallGraph() {
	for uri := range wa.documents {
		wa.callGraph[uri] = nil
	}
	for uri, result := range wa.documents {
		for _, target := range result.CallTargets {
			if resolved := wa.resolveCallTarget(uri, target); resolved != "" {
				wa.callGraph[uri] = append(wa.callGraph[uri], resolved)
			}
		}
	}
}

func (wa *WorkspaceAnalyzer) RemoveDocument(uri string) {
	delete(wa.documents, uri)
	delete(wa.callGraph, uri)

	for u, targets := range wa.callGraph {
		newTargets := []string{}
		for _, t := range targets {
			if t != uri {
				newTargets = append(newTargets, t)
			}
		}
		wa.callGraph[u] = newTargets
	}
}

func (wa *WorkspaceAnalyzer) buildCallGraph() {
	for uri, result := range wa.documents {
		wa.callGraph[uri] = []string{}
		for _, target := range result.CallTargets {
			if resolved := wa.resolveCallTarget(uri, target); resolved != "" {
				wa.callGraph[uri] = append(wa.callGraph[uri], resolved)
			}
		}
	}
}

func (wa *WorkspaceAnalyzer) resolveCallTarget(fromURI, target string) string {
	targetLower := strings.ToLower(target)

	if strings.ContainsAny(target, "/\\") {
		for uri := range wa.documents {
			if strings.HasSuffix(strings.ToLower(uri), targetLower) {
				return uri
			}
		}
	}

	fromDir := filepath.Dir(fromURI)
	candidate := filepath.Join(fromDir, target)
	candidateLower := strings.ToLower(candidate)

	for uri := range wa.documents {
		if strings.ToLower(uri) == candidateLower {
			return uri
		}
	}

	for uri := range wa.documents {
		uriBase := filepath.Base(uri)
		if strings.ToLower(uriBase) == targetLower {
			return uri
		}
	}

	return ""
}

func (wa *WorkspaceAnalyzer) propagateVariables() {
	for uri, result := range wa.documents {
		calledURIs := wa.GetCalledURIs(uri)
		for _, calledURI := range calledURIs {
			if calledResult, ok := wa.documents[calledURI]; ok {
				for name, sym := range calledResult.Symbols.Vars {
					if existing, ok := result.Symbols.Vars[name]; !ok {
						result.Symbols.Vars[name] = sym
					} else {
						for _, ref := range sym.References {
							existing.References = append(existing.References, ref)
						}
					}
				}
			}
		}
	}
}

func (wa *WorkspaceAnalyzer) GetCalledURIs(uri string) []string {
	return wa.callGraph[uri]
}

func (wa *WorkspaceAnalyzer) GetCallerURIs(uri string) []string {
	var callers []string
	for u, targets := range wa.callGraph {
		if slices.Contains(targets, uri) {
			callers = append(callers, u)
		}
	}
	return callers
}

func (wa *WorkspaceAnalyzer) GetDocument(uri string) *Result {
	return wa.documents[uri]
}

func (wa *WorkspaceAnalyzer) GetInheritedVariables(uri string) map[string]*Symbol {
	vars := make(map[string]*Symbol)

	callers := wa.GetCallerURIs(uri)
	for _, callerURI := range callers {
		if callerResult, ok := wa.documents[callerURI]; ok {
			for name, sym := range callerResult.Symbols.Vars {
				if _, exists := vars[name]; !exists {
					vars[name] = sym
				}
			}
		}
	}

	return vars
}
