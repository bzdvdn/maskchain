package dictionary

// @sk-task 24-shield-dictionaries#T1.2: Implement WordlistMatcher Aho-Corasick (AC-008)
type WordlistMatcher struct {
	trie   *trieNode
	output map[*trieNode][]string
}

type trieNode struct {
	children map[byte]*trieNode
	fail     *trieNode
}

type Match struct {
	Pattern string
	Start   int
	End     int
}

func BuildWordlistMatcher(entries []string) *WordlistMatcher {
	root := &trieNode{children: make(map[byte]*trieNode)}

	// build trie
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		node := root
		for i := 0; i < len(entry); i++ {
			c := entry[i]
			child, ok := node.children[c]
			if !ok {
				child = &trieNode{children: make(map[byte]*trieNode)}
				node.children[c] = child
			}
			node = child
		}
	}

	// build failure links + output propagation (BFS)
	output := make(map[*trieNode][]string)
	queue := make([]*trieNode, 0, len(root.children))

	for _, child := range root.children {
		child.fail = root
		queue = append(queue, child)
	}

	for _, entry := range entries {
		if entry == "" {
			continue
		}
		node := root
		for i := 0; i < len(entry); i++ {
			child := node.children[entry[i]]
			if child == nil {
				break
			}
			node = child
		}
		output[node] = append(output[node], entry)
	}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		for c, child := range node.children {
			fail := node.fail
			for fail != nil {
				if next, ok := fail.children[c]; ok {
					child.fail = next
					break
				}
				fail = fail.fail
			}
			if child.fail == nil {
				child.fail = root
			}
			// propagate output from fail node
			if child.fail != nil {
				output[child] = append(output[child], output[child.fail]...)
			}
			queue = append(queue, child)
		}
	}

	return &WordlistMatcher{trie: root, output: output}
}

func (m *WordlistMatcher) Match(text string) []Match {
	var matches []Match
	node := m.trie

	for i := 0; i < len(text); i++ {
		c := text[i]

		for node != m.trie {
			if _, ok := node.children[c]; ok {
				break
			}
			node = node.fail
		}

		if next, ok := node.children[c]; ok {
			node = next
		}

		if patterns, ok := m.output[node]; ok {
			for _, pat := range patterns {
				matches = append(matches, Match{
					Pattern: pat,
					Start:   i + 1 - len(pat),
					End:     i + 1,
				})
			}
		}
	}

	return matches
}
