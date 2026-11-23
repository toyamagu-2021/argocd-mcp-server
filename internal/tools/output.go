package tools

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	lastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"
	defaultPageLimit      = 50
	defaultManifestLimit  = 10
	defaultTailLines      = 20
)

// page is the paged tool-result object. Notice is omitted when empty.
// Hosts is not a field of page. The resource tree embeds page and adds Hosts.
// A nil Items value marshals as null, so callers pass an empty slice for an empty page.
type page struct {
	Items      any    `json:"items"`
	TotalItems int    `json:"totalItems"`
	HasMore    bool   `json:"hasMore"`
	Notice     string `json:"notice,omitempty"`
}

// pageArgs reads limit and offset.
// A missing or null argument uses defaultLimit for limit and 0 for offset.
// JSON whole numbers are accepted. Fractions and negatives are rejected.
// limit must be >= 1. There is no 0 = all. offset must be >= 0.
func pageArgs(request mcp.CallToolRequest, defaultLimit int) (int, int, error) {
	limit, err := wholeIntArg(request, "limit", defaultLimit)
	if err != nil {
		return 0, 0, err
	}
	if limit < 1 {
		return 0, 0, fmt.Errorf("limit must be at least 1")
	}
	offset, err := wholeIntArg(request, "offset", 0)
	if err != nil {
		return 0, 0, err
	}
	return limit, offset, nil
}

// wholeIntArg is the numeric read pageArgs uses for one argument.
// Missing or null returns def. int and int64 reject negatives with
// "must be zero or greater". float64 and every other type reject
// fractions and negatives with "must be a non-negative integer".
func wholeIntArg(request mcp.CallToolRequest, key string, def int) (int, error) {
	val, ok := request.GetArguments()[key]
	if !ok || val == nil {
		return def, nil
	}
	switch n := val.(type) {
	case int:
		if n < 0 {
			return 0, fmt.Errorf("%s must be zero or greater", key)
		}
		return n, nil
	case int64:
		if n < 0 {
			return 0, fmt.Errorf("%s must be zero or greater", key)
		}
		return int(n), nil
	case float64:
		if n < 0 || n != float64(int64(n)) {
			return 0, fmt.Errorf("%s must be a non-negative integer", key)
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
}

// pageSlice returns items[offset:offset+limit], clamped to the slice.
// It does not drop nils. Callers that need a nil drop filter first.
// The offset used for the slice is a local copy. Callers pass the original
// offset to pageNotice. Precondition: limit >= 1 and offset >= 0, which
// pageArgs already enforced.
func pageSlice[T any](items []T, limit, offset int) ([]T, int, bool) {
	total := len(items)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return items[offset:end], total, end < total
}

// pageNotice is the only sentence for a known total.
// filters is empty, or a noun list such as
// "kind, resource_name, resource_namespace, group, or version".
// The range is 0-based and inclusive: offset through offset+returned-1.
// "Showing items 1-2 of 3" is offset 1, returned 2.
//
//	offset == 0 && !hasMore          -> ""
//	returned == 0 && filters == ""   -> "offset %d is past %d items."
//	returned == 0                    -> "offset %d is past %d items. Narrow with %s."
//	hasMore && filters == ""         -> "Showing items %d-%d of %d. Pass offset=%d to fetch more."
//	hasMore                          -> "Showing items %d-%d of %d. Pass offset=%d to fetch more, or narrow with %s."
//	otherwise                        -> "Showing items %d-%d of %d."
func pageNotice(offset, returned, total int, hasMore bool, filters string) string {
	if offset == 0 && !hasMore {
		return ""
	}
	if returned == 0 {
		if filters == "" {
			return fmt.Sprintf("offset %d is past %d items.", offset, total)
		}
		return fmt.Sprintf("offset %d is past %d items. Narrow with %s.", offset, total, filters)
	}
	end := offset + returned
	if hasMore {
		if filters == "" {
			return fmt.Sprintf("Showing items %d-%d of %d. Pass offset=%d to fetch more.", offset, end-1, total, end)
		}
		return fmt.Sprintf("Showing items %d-%d of %d. Pass offset=%d to fetch more, or narrow with %s.", offset, end-1, total, end, filters)
	}
	return fmt.Sprintf("Showing items %d-%d of %d.", offset, end-1, total)
}

// tailNotice is the log sentence. The RPC does not report a total.
// returned < requested yields "". Otherwise the sentence names the
// returned line count and the requested tail_lines.
func tailNotice(returned, requested int) string {
	if returned < requested {
		return ""
	}
	return fmt.Sprintf("Showing the last %d lines (tail_lines=%d). The server does not report the total. Pass a larger tail_lines to see more.", returned, requested)
}

// writeCompact marshals v with json.Marshal and returns the bytes as text.
// A marshal failure is a tool error: "Failed to format response: %v".
func writeCompact(v any) (*mcp.CallToolResult, error) {
	jsonData, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}

// stripAnnotations deletes the last-applied-configuration annotation.
// It returns nil when no annotations remain.
func stripAnnotations(ann map[string]string) map[string]string {
	if len(ann) == 0 {
		return nil
	}
	delete(ann, lastAppliedAnnotation)
	if len(ann) == 0 {
		return nil
	}
	return ann
}

// stripObjectMeta clears managedFields and the last-applied annotation.
// A nil meta is left alone.
func stripObjectMeta(meta *metav1.ObjectMeta) {
	if meta == nil {
		return
	}
	meta.ManagedFields = nil
	meta.Annotations = stripAnnotations(meta.Annotations)
}

// stripManifestNoise removes metadata.managedFields and the last-applied
// annotation from a JSON object manifest. Decode failure returns raw.
// Non-object JSON, including arrays, is returned unchanged.
func stripManifestNoise(raw string) string {
	if raw == "" {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return raw
	}
	if meta, ok := obj["metadata"].(map[string]any); ok {
		delete(meta, "managedFields")
		if ann, ok := meta["annotations"].(map[string]any); ok {
			delete(ann, lastAppliedAnnotation)
			if len(ann) == 0 {
				delete(meta, "annotations")
			}
		}
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return string(out)
}
