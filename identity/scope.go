package identity

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// GlobMatch matches slash-normalized resources. '*' does not cross a slash,
// while '**' does. '?' matches one non-slash rune. Backslashes are normalized
// to slashes so snapshots behave consistently across operating systems.
func GlobMatch(pattern, value string) bool {
	pattern = strings.TrimSpace(strings.ReplaceAll(pattern, "\\", "/"))
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if pattern == "" {
		return value == ""
	}
	type state struct{ p, v int }
	memo := make(map[state]bool)
	seen := make(map[state]bool)
	var match func(int, int) bool
	match = func(pi, vi int) bool {
		key := state{pi, vi}
		if seen[key] {
			return memo[key]
		}
		seen[key] = true
		if pi == len(pattern) {
			memo[key] = vi == len(value)
			return memo[key]
		}
		pr, ps := utf8.DecodeRuneInString(pattern[pi:])
		switch pr {
		case '*':
			next := pi + ps
			double := next < len(pattern) && pattern[next] == '*'
			if double {
				for next < len(pattern) && pattern[next] == '*' {
					next++
				}
			}
			if match(next, vi) {
				memo[key] = true
				return true
			}

			for cursor := vi; cursor < len(value); {
				vr, vs := utf8.DecodeRuneInString(value[cursor:])
				if !double && vr == '/' {
					break
				}
				cursor += vs
				if match(next, cursor) {
					memo[key] = true
					return true
				}
			}
		case '?':
			if vi < len(value) {
				vr, vs := utf8.DecodeRuneInString(value[vi:])
				memo[key] = vr != '/' && match(pi+ps, vi+vs)
				return memo[key]
			}
		default:
			if vi < len(value) {
				vr, vs := utf8.DecodeRuneInString(value[vi:])
				memo[key] = pr == vr && match(pi+ps, vi+vs)
				return memo[key]
			}
		}
		memo[key] = false
		return false
	}
	return match(0, 0)
}

// MatchResourceScope reports whether a resource is within at least one scope.
// An empty scope list is unrestricted.
func MatchResourceScope(scopes []string, resource string) bool {
	return MatchScope(scopes, resource)
}

// MatchToolScope reports whether a tool name is within at least one scope.
func MatchToolScope(scopes []string, tool string) bool {
	return MatchScope(scopes, tool)
}

// MatchScope reports whether value matches one normalized glob.
func MatchScope(scopes []string, value string) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, scope := range scopes {
		if GlobMatch(scope, value) {
			return true
		}
	}
	return false
}

// scopeSetContained conservatively establishes that every child pattern is
// bounded by some parent pattern. It deliberately rejects ambiguous narrowing.
func scopeSetContained(parent, child []string) bool {
	if len(parent) == 0 {
		return true
	}
	if len(child) == 0 {
		return false
	}
	for _, candidate := range child {
		covered := false
		for _, boundary := range parent {
			if patternContains(boundary, candidate) {
				covered = true
				break
			}
		}
		if !covered {
			return false
		}
	}
	return true
}

// patternContains reports whether every value matched by child is also matched
// by parent (the child language is a subset of the parent language). It is
// sound: it only returns true when containment is provable, and deliberately
// rejects wildcard children whose relationship to the parent is broadening or
// semantically uncertain. Literal children and genuinely narrowing glob
// delegations (for example repos/acme/docs/** under repos/acme/**) are
// preserved.
func patternContains(parent, child string) bool {
	parent = strings.ReplaceAll(strings.TrimSpace(parent), "\\", "/")
	child = strings.ReplaceAll(strings.TrimSpace(child), "\\", "/")
	if parent == child || parent == "**" {
		return true
	}
	if !strings.ContainsAny(child, "*?") {
		// Literal child: the parent must match that concrete value.
		return GlobMatch(parent, child)
	}
	return scopeSubset(parent, child)
}

// scopeSubset decides containment for glob children by comparing the path
// segment structure. A trailing "**" segment matches the rest of the path; any
// other segment matches exactly one path component. Patterns it cannot reason
// about soundly (such as "**" anywhere except as the final segment) are
// rejected as uncertain.
func scopeSubset(parent, child string) bool {
	parentLeaves, parentTail, ok := splitScope(parent)
	if !ok {
		return false
	}
	childLeaves, childTail, ok := splitScope(child)
	if !ok {
		return false
	}
	// A bare "**" parent matches everything.
	if len(parentLeaves) == 0 && parentTail {
		return true
	}
	switch {
	case !parentTail && !childTail:
		// Both match a fixed number of segments.
		if len(parentLeaves) != len(childLeaves) {
			return false
		}
		return leavesCover(parentLeaves, childLeaves)
	case parentTail && !childTail:
		// The parent spans a suffix, so the child's exact segments must extend
		// one segment past the parent's prefix (the "/" that precedes "**").
		if len(childLeaves) < len(parentLeaves)+1 {
			return false
		}
		return leavesCover(parentLeaves, childLeaves)
	case !parentTail && childTail:
		// The child spans arbitrary depth a fixed-length parent cannot cover.
		return false
	default: // parentTail && childTail
		// Both span a suffix; the parent's prefix must be no deeper than the
		// child's so the parent's "**" absorbs the child's remainder.
		if len(childLeaves) < len(parentLeaves) {
			return false
		}
		return leavesCover(parentLeaves, childLeaves)
	}
}

// splitScope decomposes a scope into its leading single-segment components and
// reports whether it ends with a recursive "**" tail. It rejects patterns it
// cannot reason about soundly: "**" anywhere except as the final segment, or a
// "**" mixed into another segment.
func splitScope(pattern string) (leaves []string, hasTail bool, ok bool) {
	if pattern == "" {
		return nil, false, false
	}
	segments := strings.Split(pattern, "/")
	for i, segment := range segments {
		if segment == "**" {
			if i != len(segments)-1 {
				return nil, false, false
			}
			return segments[:i], true, true
		}
		if strings.Contains(segment, "**") {
			return nil, false, false
		}
	}
	return segments, false, true
}

// leavesCover reports whether each parent segment bounds the corresponding
// child segment.
func leavesCover(parent, child []string) bool {
	for i := range parent {
		if !leafCovers(parent[i], child[i]) {
			return false
		}
	}
	return true
}

// leafCovers reports whether every single-segment value matched by child is
// also matched by parent. Two distinct wildcard segments have an uncertain
// relationship and are rejected.
func leafCovers(parent, child string) bool {
	if parent == child || parent == "*" {
		return true
	}
	if !strings.ContainsAny(child, "*?") {
		return GlobMatch(parent, child)
	}
	return false
}

func validateScopes(scopes []string, name string) error {
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == "" {
			return wrap(ErrInvalid, "%s scope is empty", name)
		}
		if !utf8.ValidString(scope) {
			return fmt.Errorf("%w: %s scope is not UTF-8", ErrInvalid, name)
		}
	}
	return nil
}
