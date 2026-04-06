package strmatcher

import (
	"container/list"
)

const validCharCount = 53

type MatchType struct {
	Type  Type
	Exist bool
}

const (
	TrieEdge bool = true
	FailEdge bool = false
)

type Edge struct {
	Type     bool
	NextNode int
}

type ACAutomaton struct {
	Trie   [][validCharCount]Edge
	Fail   []int
	Exists []MatchType
	Count  int
}

func newNode() [validCharCount]Edge {
	var s [validCharCount]Edge
	for i := range s {
		s[i] = Edge{
			Type:     FailEdge,
			NextNode: 0,
		}
	}
	return s
}

var char2Index = []int{
	'A':  0,
	'a':  0,
	'B':  1,
	'b':  1,
	'C':  2,
	'c':  2,
	'D':  3,
	'd':  3,
	'E':  4,
	'e':  4,
	'F':  5,
	'f':  5,
	'G':  6,
	'g':  6,
	'H':  7,
	'h':  7,
	'I':  8,
	'i':  8,
	'J':  9,
	'j':  9,
	'K':  10,
	'k':  10,
	'L':  11,
	'l':  11,
	'M':  12,
	'm':  12,
	'N':  13,
	'n':  13,
	'O':  14,
	'o':  14,
	'P':  15,
	'p':  15,
	'Q':  16,
	'q':  16,
	'R':  17,
	'r':  17,
	'S':  18,
	's':  18,
	'T':  19,
	't':  19,
	'U':  20,
	'u':  20,
	'V':  21,
	'v':  21,
	'W':  22,
	'w':  22,
	'X':  23,
	'x':  23,
	'Y':  24,
	'y':  24,
	'Z':  25,
	'z':  25,
	'!':  26,
	'$':  27,
	'&':  28,
	'\'': 29,
	'(':  30,
	')':  31,
	'*':  32,
	'+':  33,
	',':  34,
	';':  35,
	'=':  36,
	':':  37,
	'%':  38,
	'-':  39,
	'.':  40,
	'_':  41,
	'~':  42,
	'0':  43,
	'1':  44,
	'2':  45,
	'3':  46,
	'4':  47,
	'5':  48,
	'6':  49,
	'7':  50,
	'8':  51,
	'9':  52,
}

func NewACAutomaton() *ACAutomaton {
	ac := new(ACAutomaton)
	ac.Trie = append(ac.Trie, newNode())
	ac.Fail = append(ac.Fail, 0)
	ac.Exists = append(ac.Exists, MatchType{
		Type:  Full,
		Exist: false,
	})
	return ac
}

func (ac *ACAutomaton) Add(domain string, t Type) {
	node := 0
	for i := len(domain) - 1; i >= 0; i-- {
		idx := char2Index[domain[i]]
		if ac.Trie[node][idx].NextNode == 0 {
			ac.Count++
			if len(ac.Trie) < ac.Count+1 {
				ac.Trie = append(ac.Trie, newNode())
				ac.Fail = append(ac.Fail, 0)
				ac.Exists = append(ac.Exists, MatchType{
					Type:  Full,
					Exist: false,
				})
			}
			ac.Trie[node][idx] = Edge{
				Type:     TrieEdge,
				NextNode: ac.Count,
			}
		}
		node = ac.Trie[node][idx].NextNode
	}
	ac.Exists[node] = MatchType{
		Type:  t,
		Exist: true,
	}
	switch t {
	case Domain:
		ac.Exists[node] = MatchType{
			Type:  Full,
			Exist: true,
		}
		idx := char2Index['.']
		if ac.Trie[node][idx].NextNode == 0 {
			ac.Count++
			if len(ac.Trie) < ac.Count+1 {
				ac.Trie = append(ac.Trie, newNode())
				ac.Fail = append(ac.Fail, 0)
				ac.Exists = append(ac.Exists, MatchType{
					Type:  Full,
					Exist: false,
				})
			}
			ac.Trie[node][idx] = Edge{
				Type:     TrieEdge,
				NextNode: ac.Count,
			}
		}
		node = ac.Trie[node][idx].NextNode
		ac.Exists[node] = MatchType{
			Type:  t,
			Exist: true,
		}
	default:
		break
	}
}

func (ac *ACAutomaton) Build() {
	queue := list.New()
	for i := 0; i < validCharCount; i++ {
		if ac.Trie[0][i].NextNode != 0 {
			queue.PushBack(ac.Trie[0][i])
		}
	}
	for {
		front := queue.Front()
		if front == nil {
			break
		} else {
			node := front.Value.(Edge).NextNode
			queue.Remove(front)
			for i := 0; i < validCharCount; i++ {
				if ac.Trie[node][i].NextNode != 0 {
					ac.Fail[ac.Trie[node][i].NextNode] = ac.Trie[ac.Fail[node]][i].NextNode
					queue.PushBack(ac.Trie[node][i])
				} else {
					ac.Trie[node][i] = Edge{
						Type:     FailEdge,
						NextNode: ac.Trie[ac.Fail[node]][i].NextNode,
					}
				}
			}
		}
	}
}

func (ac *ACAutomaton) Match(s string) bool {
	node := 0
	fullMatch := true
	// 1. the match string is all through trie edge. FULL MATCH or DOMAIN
	// 2. the match string is through a fail edge. NOT FULL MATCH
	// 2.1 Through a fail edge, but there exists a valid node. SUBSTR
	for i := len(s) - 1; i >= 0; i-- {
		chr := int(s[i])
		if chr >= len(char2Index) {
			return false
		}
		idx := char2Index[chr]
		fullMatch = fullMatch && ac.Trie[node][idx].Type
		node = ac.Trie[node][idx].NextNode
		switch ac.Exists[node].Type {
		case Substr:
			return true
		case Domain:
			if fullMatch {
				return true
			}
		default:
			break
		}
	}
	return fullMatch && ac.Exists[node].Exist
}
