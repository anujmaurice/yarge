package rangeexpr

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const end_symbol rune = 4

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	rulee
	rulecombinedexpr
	rulerexpr
	rulecexpr
	ruleunion
	ruleintersection
	ruledifference
	rulecluster
	rulekey
	rulerlookup
	ruleattr
	rulehint
	rulevalue
	rulefirst
	rulemiddle
	rulelast
	rulebrackets
	rulesp
	ruleAction0
	ruleAction1
	ruleAction2
	rulePegText
	ruleAction3
	ruleAction4
	ruleAction5
	ruleAction6
	ruleAction7
	ruleAction8
	ruleAction9

	rulePre_
	rule_In_
	rule_Suf
)

var rul3s = [...]string{
	"Unknown",
	"e",
	"combinedexpr",
	"rexpr",
	"cexpr",
	"union",
	"intersection",
	"difference",
	"cluster",
	"key",
	"rlookup",
	"attr",
	"hint",
	"value",
	"first",
	"middle",
	"last",
	"brackets",
	"sp",
	"Action0",
	"Action1",
	"Action2",
	"PegText",
	"Action3",
	"Action4",
	"Action5",
	"Action6",
	"Action7",
	"Action8",
	"Action9",

	"Pre_",
	"_In_",
	"_Suf",
}

type tokenTree interface {
	Print()
	PrintSyntax()
	PrintSyntaxTree(buffer string)
	Add(rule pegRule, begin, end, next uint32, depth int)
	Expand(index int) tokenTree
	Tokens() <-chan token32
	AST() *node32
	Error() []token32
	trim(length int)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(depth int, buffer string) {
	for node != nil {
		for c := 0; c < depth; c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[node.pegRule], strconv.Quote(string(([]rune(buffer)[node.begin:node.end]))))
		if node.up != nil {
			node.up.print(depth+1, buffer)
		}
		node = node.next
	}
}

func (ast *node32) Print(buffer string) {
	ast.print(0, buffer)
}

type element struct {
	node *node32
	down *element
}

/* ${@} bit structure for abstract syntax tree */
type token32 struct {
	pegRule
	begin, end, next uint32
}

func (t *token32) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token32) isParentOf(u token32) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token32) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: uint32(t.begin), end: uint32(t.end), next: uint32(t.next)}
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens32 struct {
	tree    []token32
	ordered [][]token32
}

func (t *tokens32) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) Order() [][]token32 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int32, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token32, len(depths)), make([]token32, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = uint32(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state32 struct {
	token32
	depths []int32
	leaf   bool
}

func (t *tokens32) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens32) PreOrder() (<-chan state32, [][]token32) {
	s, ordered := make(chan state32, 6), t.Order()
	go func() {
		var states [8]state32
		for i, _ := range states {
			states[i].depths = make([]int32, len(ordered))
		}
		depths, state, depth := make([]int32, len(ordered)), 0, 1
		write := func(t token32, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, uint32(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token32 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token32{pegRule: rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token32{pegRule: rulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token32{pegRule: rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens32) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(string(([]rune(buffer)[token.begin:token.end]))))
	}
}

func (t *tokens32) Add(rule pegRule, begin, end, depth uint32, index int) {
	t.tree[index] = token32{pegRule: rule, begin: uint32(begin), end: uint32(end), next: uint32(depth)}
}

func (t *tokens32) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens32) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

/*func (t *tokens16) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2 * len(tree))
		for i, v := range tree {
			expanded[i] = v.getToken32()
		}
		return &tokens32{tree: expanded}
	}
	return nil
}*/

func (t *tokens32) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	return nil
}

type RangeExpr struct {
	Expression

	Buffer string
	buffer []rune
	rules  [30]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	tokenTree
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer string, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer[0:] {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p *RangeExpr
}

func (e *parseError) Error() string {
	tokens, error := e.p.tokenTree.Error(), "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.Buffer, positions)
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf("parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n",
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			/*strconv.Quote(*/ e.p.Buffer[begin:end] /*)*/)
	}

	return error
}

func (p *RangeExpr) PrintSyntaxTree() {
	p.tokenTree.PrintSyntaxTree(p.Buffer)
}

func (p *RangeExpr) Highlighter() {
	p.tokenTree.PrintSyntax()
}

func (p *RangeExpr) Execute() {
	buffer, begin, end := p.Buffer, 0, 0
	for token := range p.tokenTree.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)

		case ruleAction0:
			p.addOperator(typeUnion)
		case ruleAction1:
			p.addOperator(typeIntersection)
		case ruleAction2:
			p.addOperator(typeDifference)
		case ruleAction3:
			p.addValue(buffer[begin:end])
		case ruleAction4:
			p.addOperator(typeClusterLookup)
		case ruleAction5:
			p.addValue(buffer[begin:end])
			p.addOperator(typeKeyLookup)
		case ruleAction6:
			p.addValue(buffer[begin:end])
			p.addOperator(typeKeyReverseLookup)
		case ruleAction7:
			p.addValue(buffer[begin:end])
			p.addOperator(typeKeyReverseLookupAttr)
		case ruleAction8:
			p.addOperator(typeKeyReverseLookupHint)
		case ruleAction9:
			p.addValue(buffer[begin:end])

		}
	}
	_, _, _ = buffer, begin, end
}

func (p *RangeExpr) Init() {
	p.buffer = []rune(p.Buffer)
	if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != end_symbol {
		p.buffer = append(p.buffer, end_symbol)
	}

	var tree tokenTree = &tokens32{tree: make([]token32, math.MaxInt16)}
	position, depth, tokenIndex, buffer, _rules := uint32(0), uint32(0), 0, p.buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokenTree = tree
		if matches {
			p.tokenTree.trim(tokenIndex)
			return nil
		}
		return &parseError{p}
	}

	p.Reset = func() {
		position, tokenIndex, depth = 0, 0, 0
	}

	add := func(rule pegRule, begin uint32) {
		if t := tree.Expand(tokenIndex); t != nil {
			tree = t
		}
		tree.Add(rule, begin, position, depth, tokenIndex)
		tokenIndex++
	}

	matchDot := func() bool {
		if buffer[position] != end_symbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	_rules = [...]func() bool{
		nil,
		/* 0 e <- <(combinedexpr? !.)> */
		func() bool {
			position0, tokenIndex0, depth0 := position, tokenIndex, depth
			{
				position1 := position
				depth++
				{
					position2, tokenIndex2, depth2 := position, tokenIndex, depth
					if !_rules[rulecombinedexpr]() {
						goto l2
					}
					goto l3
				l2:
					position, tokenIndex, depth = position2, tokenIndex2, depth2
				}
			l3:
				{
					position4, tokenIndex4, depth4 := position, tokenIndex, depth
					if !matchDot() {
						goto l4
					}
					goto l0
				l4:
					position, tokenIndex, depth = position4, tokenIndex4, depth4
				}
				depth--
				add(rulee, position1)
			}
			return true
		l0:
			position, tokenIndex, depth = position0, tokenIndex0, depth0
			return false
		},
		/* 1 combinedexpr <- <(rexpr cexpr?)> */
		func() bool {
			position5, tokenIndex5, depth5 := position, tokenIndex, depth
			{
				position6 := position
				depth++
				if !_rules[rulerexpr]() {
					goto l5
				}
				{
					position7, tokenIndex7, depth7 := position, tokenIndex, depth
					if !_rules[rulecexpr]() {
						goto l7
					}
					goto l8
				l7:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
				}
			l8:
				depth--
				add(rulecombinedexpr, position6)
			}
			return true
		l5:
			position, tokenIndex, depth = position5, tokenIndex5, depth5
			return false
		},
		/* 2 rexpr <- <(sp (cluster / brackets / value / rlookup))> */
		func() bool {
			position9, tokenIndex9, depth9 := position, tokenIndex, depth
			{
				position10 := position
				depth++
				if !_rules[rulesp]() {
					goto l9
				}
				{
					position11, tokenIndex11, depth11 := position, tokenIndex, depth
					{
						position13 := position
						depth++
						{
							position14, tokenIndex14, depth14 := position, tokenIndex, depth
							if buffer[position] != rune('%') {
								goto l15
							}
							position++
							{
								position16 := position
								depth++
								if buffer[position] != rune('R') {
									goto l15
								}
								position++
								if buffer[position] != rune('A') {
									goto l15
								}
								position++
								if buffer[position] != rune('N') {
									goto l15
								}
								position++
								if buffer[position] != rune('G') {
									goto l15
								}
								position++
								if buffer[position] != rune('E') {
									goto l15
								}
								position++
								depth--
								add(rulePegText, position16)
							}
							{
								add(ruleAction3, position)
							}
							goto l14
						l15:
							position, tokenIndex, depth = position14, tokenIndex14, depth14
							if buffer[position] != rune('%') {
								goto l12
							}
							position++
							if !_rules[rulerexpr]() {
								goto l12
							}
						}
					l14:
						{
							add(ruleAction4, position)
						}
						{
							position19, tokenIndex19, depth19 := position, tokenIndex, depth
							{
								position21 := position
								depth++
								if buffer[position] != rune(':') {
									goto l19
								}
								position++
								{
									position22 := position
									depth++
									{
										position25, tokenIndex25, depth25 := position, tokenIndex, depth
										if c := buffer[position]; c < rune('A') || c > rune('Z') {
											goto l26
										}
										position++
										goto l25
									l26:
										position, tokenIndex, depth = position25, tokenIndex25, depth25
										if c := buffer[position]; c < rune('0') || c > rune('9') {
											goto l19
										}
										position++
									}
								l25:
								l23:
									{
										position24, tokenIndex24, depth24 := position, tokenIndex, depth
										{
											position27, tokenIndex27, depth27 := position, tokenIndex, depth
											if c := buffer[position]; c < rune('A') || c > rune('Z') {
												goto l28
											}
											position++
											goto l27
										l28:
											position, tokenIndex, depth = position27, tokenIndex27, depth27
											if c := buffer[position]; c < rune('0') || c > rune('9') {
												goto l24
											}
											position++
										}
									l27:
										goto l23
									l24:
										position, tokenIndex, depth = position24, tokenIndex24, depth24
									}
									depth--
									add(rulePegText, position22)
								}
								{
									add(ruleAction5, position)
								}
								depth--
								add(rulekey, position21)
							}
							goto l20
						l19:
							position, tokenIndex, depth = position19, tokenIndex19, depth19
						}
					l20:
						depth--
						add(rulecluster, position13)
					}
					goto l11
				l12:
					position, tokenIndex, depth = position11, tokenIndex11, depth11
					{
						position31 := position
						depth++
						if buffer[position] != rune('(') {
							goto l30
						}
						position++
						if !_rules[rulecombinedexpr]() {
							goto l30
						}
						if buffer[position] != rune(')') {
							goto l30
						}
						position++
						depth--
						add(rulebrackets, position31)
					}
					goto l11
				l30:
					position, tokenIndex, depth = position11, tokenIndex11, depth11
					if !_rules[rulevalue]() {
						goto l32
					}
					goto l11
				l32:
					position, tokenIndex, depth = position11, tokenIndex11, depth11
					{
						position33 := position
						depth++
						if buffer[position] != rune('*') {
							goto l9
						}
						position++
						{
							position34 := position
							depth++
							{
								position37, tokenIndex37, depth37 := position, tokenIndex, depth
								if c := buffer[position]; c < rune('a') || c > rune('z') {
									goto l38
								}
								position++
								goto l37
							l38:
								position, tokenIndex, depth = position37, tokenIndex37, depth37
								if c := buffer[position]; c < rune('A') || c > rune('Z') {
									goto l39
								}
								position++
								goto l37
							l39:
								position, tokenIndex, depth = position37, tokenIndex37, depth37
								{
									position41, tokenIndex41, depth41 := position, tokenIndex, depth
									if c := buffer[position]; c < rune('0') || c > rune('9') {
										goto l42
									}
									position++
									goto l41
								l42:
									position, tokenIndex, depth = position41, tokenIndex41, depth41
									if c := buffer[position]; c < rune('0') || c > rune('9') {
										goto l40
									}
									position++
								}
							l41:
								goto l37
							l40:
								position, tokenIndex, depth = position37, tokenIndex37, depth37
								if buffer[position] != rune(' ') {
									goto l43
								}
								position++
								goto l37
							l43:
								position, tokenIndex, depth = position37, tokenIndex37, depth37
								if buffer[position] != rune('.') {
									goto l9
								}
								position++
							}
						l37:
						l35:
							{
								position36, tokenIndex36, depth36 := position, tokenIndex, depth
								{
									position44, tokenIndex44, depth44 := position, tokenIndex, depth
									if c := buffer[position]; c < rune('a') || c > rune('z') {
										goto l45
									}
									position++
									goto l44
								l45:
									position, tokenIndex, depth = position44, tokenIndex44, depth44
									if c := buffer[position]; c < rune('A') || c > rune('Z') {
										goto l46
									}
									position++
									goto l44
								l46:
									position, tokenIndex, depth = position44, tokenIndex44, depth44
									{
										position48, tokenIndex48, depth48 := position, tokenIndex, depth
										if c := buffer[position]; c < rune('0') || c > rune('9') {
											goto l49
										}
										position++
										goto l48
									l49:
										position, tokenIndex, depth = position48, tokenIndex48, depth48
										if c := buffer[position]; c < rune('0') || c > rune('9') {
											goto l47
										}
										position++
									}
								l48:
									goto l44
								l47:
									position, tokenIndex, depth = position44, tokenIndex44, depth44
									if buffer[position] != rune(' ') {
										goto l50
									}
									position++
									goto l44
								l50:
									position, tokenIndex, depth = position44, tokenIndex44, depth44
									if buffer[position] != rune('.') {
										goto l36
									}
									position++
								}
							l44:
								goto l35
							l36:
								position, tokenIndex, depth = position36, tokenIndex36, depth36
							}
							depth--
							add(rulePegText, position34)
						}
						{
							add(ruleAction6, position)
						}
						{
							position52, tokenIndex52, depth52 := position, tokenIndex, depth
							{
								position54 := position
								depth++
								if buffer[position] != rune(';') {
									goto l52
								}
								position++
								{
									position55 := position
									depth++
									{
										position58, tokenIndex58, depth58 := position, tokenIndex, depth
										if c := buffer[position]; c < rune('A') || c > rune('Z') {
											goto l59
										}
										position++
										goto l58
									l59:
										position, tokenIndex, depth = position58, tokenIndex58, depth58
										if c := buffer[position]; c < rune('0') || c > rune('9') {
											goto l52
										}
										position++
									}
								l58:
								l56:
									{
										position57, tokenIndex57, depth57 := position, tokenIndex, depth
										{
											position60, tokenIndex60, depth60 := position, tokenIndex, depth
											if c := buffer[position]; c < rune('A') || c > rune('Z') {
												goto l61
											}
											position++
											goto l60
										l61:
											position, tokenIndex, depth = position60, tokenIndex60, depth60
											if c := buffer[position]; c < rune('0') || c > rune('9') {
												goto l57
											}
											position++
										}
									l60:
										goto l56
									l57:
										position, tokenIndex, depth = position57, tokenIndex57, depth57
									}
									depth--
									add(rulePegText, position55)
								}
								{
									add(ruleAction7, position)
								}
								{
									position63, tokenIndex63, depth63 := position, tokenIndex, depth
									{
										position65 := position
										depth++
										if buffer[position] != rune(':') {
											goto l63
										}
										position++
										if !_rules[rulevalue]() {
											goto l63
										}
										{
											add(ruleAction8, position)
										}
										depth--
										add(rulehint, position65)
									}
									goto l64
								l63:
									position, tokenIndex, depth = position63, tokenIndex63, depth63
								}
							l64:
								depth--
								add(ruleattr, position54)
							}
							goto l53
						l52:
							position, tokenIndex, depth = position52, tokenIndex52, depth52
						}
					l53:
						{
							position67, tokenIndex67, depth67 := position, tokenIndex, depth
							if !_rules[rulecexpr]() {
								goto l67
							}
							goto l68
						l67:
							position, tokenIndex, depth = position67, tokenIndex67, depth67
						}
					l68:
						depth--
						add(rulerlookup, position33)
					}
				}
			l11:
				depth--
				add(rulerexpr, position10)
			}
			return true
		l9:
			position, tokenIndex, depth = position9, tokenIndex9, depth9
			return false
		},
		/* 3 cexpr <- <(sp (union / intersection / difference) sp)> */
		func() bool {
			position69, tokenIndex69, depth69 := position, tokenIndex, depth
			{
				position70 := position
				depth++
				if !_rules[rulesp]() {
					goto l69
				}
				{
					position71, tokenIndex71, depth71 := position, tokenIndex, depth
					{
						position73 := position
						depth++
						if buffer[position] != rune(',') {
							goto l72
						}
						position++
						if !_rules[rulerexpr]() {
							goto l72
						}
						{
							position74, tokenIndex74, depth74 := position, tokenIndex, depth
							if !_rules[rulecexpr]() {
								goto l74
							}
							goto l75
						l74:
							position, tokenIndex, depth = position74, tokenIndex74, depth74
						}
					l75:
						{
							add(ruleAction0, position)
						}
						depth--
						add(ruleunion, position73)
					}
					goto l71
				l72:
					position, tokenIndex, depth = position71, tokenIndex71, depth71
					{
						position78 := position
						depth++
						if buffer[position] != rune(',') {
							goto l77
						}
						position++
						if !_rules[rulesp]() {
							goto l77
						}
						if buffer[position] != rune('&') {
							goto l77
						}
						position++
						if !_rules[rulerexpr]() {
							goto l77
						}
						{
							position79, tokenIndex79, depth79 := position, tokenIndex, depth
							if !_rules[rulecexpr]() {
								goto l79
							}
							goto l80
						l79:
							position, tokenIndex, depth = position79, tokenIndex79, depth79
						}
					l80:
						{
							add(ruleAction1, position)
						}
						depth--
						add(ruleintersection, position78)
					}
					goto l71
				l77:
					position, tokenIndex, depth = position71, tokenIndex71, depth71
					{
						position82 := position
						depth++
						if buffer[position] != rune(',') {
							goto l69
						}
						position++
						if !_rules[rulesp]() {
							goto l69
						}
						if buffer[position] != rune('-') {
							goto l69
						}
						position++
						if !_rules[rulerexpr]() {
							goto l69
						}
						{
							position83, tokenIndex83, depth83 := position, tokenIndex, depth
							if !_rules[rulecexpr]() {
								goto l83
							}
							goto l84
						l83:
							position, tokenIndex, depth = position83, tokenIndex83, depth83
						}
					l84:
						{
							add(ruleAction2, position)
						}
						depth--
						add(ruledifference, position82)
					}
				}
			l71:
				if !_rules[rulesp]() {
					goto l69
				}
				depth--
				add(rulecexpr, position70)
			}
			return true
		l69:
			position, tokenIndex, depth = position69, tokenIndex69, depth69
			return false
		},
		/* 4 union <- <(',' rexpr cexpr? Action0)> */
		nil,
		/* 5 intersection <- <(',' sp '&' rexpr cexpr? Action1)> */
		nil,
		/* 6 difference <- <(',' sp '-' rexpr cexpr? Action2)> */
		nil,
		/* 7 cluster <- <((('%' <('R' 'A' 'N' 'G' 'E')> Action3) / ('%' rexpr)) Action4 key?)> */
		nil,
		/* 8 key <- <(':' <([A-Z] / [0-9])+> Action5)> */
		nil,
		/* 9 rlookup <- <('*' <([a-z] / [A-Z] / ([0-9] / [0-9]) / ' ' / '.')+> Action6 attr? cexpr?)> */
		nil,
		/* 10 attr <- <(';' <([A-Z] / [0-9])+> Action7 hint?)> */
		nil,
		/* 11 hint <- <(':' value Action8)> */
		nil,
		/* 12 value <- <(<((first last? middle+) / (first last*))> Action9)> */
		func() bool {
			position94, tokenIndex94, depth94 := position, tokenIndex, depth
			{
				position95 := position
				depth++
				{
					position96 := position
					depth++
					{
						position97, tokenIndex97, depth97 := position, tokenIndex, depth
						if !_rules[rulefirst]() {
							goto l98
						}
						{
							position99, tokenIndex99, depth99 := position, tokenIndex, depth
							if !_rules[rulelast]() {
								goto l99
							}
							goto l100
						l99:
							position, tokenIndex, depth = position99, tokenIndex99, depth99
						}
					l100:
						{
							position103 := position
							depth++
							if buffer[position] != rune('-') {
								goto l98
							}
							position++
							if !_rules[rulelast]() {
								goto l98
							}
							depth--
							add(rulemiddle, position103)
						}
					l101:
						{
							position102, tokenIndex102, depth102 := position, tokenIndex, depth
							{
								position104 := position
								depth++
								if buffer[position] != rune('-') {
									goto l102
								}
								position++
								if !_rules[rulelast]() {
									goto l102
								}
								depth--
								add(rulemiddle, position104)
							}
							goto l101
						l102:
							position, tokenIndex, depth = position102, tokenIndex102, depth102
						}
						goto l97
					l98:
						position, tokenIndex, depth = position97, tokenIndex97, depth97
						if !_rules[rulefirst]() {
							goto l94
						}
					l105:
						{
							position106, tokenIndex106, depth106 := position, tokenIndex, depth
							if !_rules[rulelast]() {
								goto l106
							}
							goto l105
						l106:
							position, tokenIndex, depth = position106, tokenIndex106, depth106
						}
					}
				l97:
					depth--
					add(rulePegText, position96)
				}
				{
					add(ruleAction9, position)
				}
				depth--
				add(rulevalue, position95)
			}
			return true
		l94:
			position, tokenIndex, depth = position94, tokenIndex94, depth94
			return false
		},
		/* 13 first <- <[a-z]+> */
		func() bool {
			position108, tokenIndex108, depth108 := position, tokenIndex, depth
			{
				position109 := position
				depth++
				if c := buffer[position]; c < rune('a') || c > rune('z') {
					goto l108
				}
				position++
			l110:
				{
					position111, tokenIndex111, depth111 := position, tokenIndex, depth
					if c := buffer[position]; c < rune('a') || c > rune('z') {
						goto l111
					}
					position++
					goto l110
				l111:
					position, tokenIndex, depth = position111, tokenIndex111, depth111
				}
				depth--
				add(rulefirst, position109)
			}
			return true
		l108:
			position, tokenIndex, depth = position108, tokenIndex108, depth108
			return false
		},
		/* 14 middle <- <('-' last)> */
		nil,
		/* 15 last <- <([a-z] / [0-9])+> */
		func() bool {
			position113, tokenIndex113, depth113 := position, tokenIndex, depth
			{
				position114 := position
				depth++
				{
					position117, tokenIndex117, depth117 := position, tokenIndex, depth
					if c := buffer[position]; c < rune('a') || c > rune('z') {
						goto l118
					}
					position++
					goto l117
				l118:
					position, tokenIndex, depth = position117, tokenIndex117, depth117
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l113
					}
					position++
				}
			l117:
			l115:
				{
					position116, tokenIndex116, depth116 := position, tokenIndex, depth
					{
						position119, tokenIndex119, depth119 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l120
						}
						position++
						goto l119
					l120:
						position, tokenIndex, depth = position119, tokenIndex119, depth119
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l116
						}
						position++
					}
				l119:
					goto l115
				l116:
					position, tokenIndex, depth = position116, tokenIndex116, depth116
				}
				depth--
				add(rulelast, position114)
			}
			return true
		l113:
			position, tokenIndex, depth = position113, tokenIndex113, depth113
			return false
		},
		/* 16 brackets <- <('(' combinedexpr ')')> */
		nil,
		/* 17 sp <- <' '*> */
		func() bool {
			{
				position123 := position
				depth++
			l124:
				{
					position125, tokenIndex125, depth125 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l125
					}
					position++
					goto l124
				l125:
					position, tokenIndex, depth = position125, tokenIndex125, depth125
				}
				depth--
				add(rulesp, position123)
			}
			return true
		},
		/* 19 Action0 <- <{ p.addOperator(typeUnion) }> */
		nil,
		/* 20 Action1 <- <{ p.addOperator(typeIntersection) }> */
		nil,
		/* 21 Action2 <- <{ p.addOperator(typeDifference) }> */
		nil,
		nil,
		/* 23 Action3 <- <{ p.addValue(buffer[begin:end]); }> */
		nil,
		/* 24 Action4 <- <{ p.addOperator(typeClusterLookup) }> */
		nil,
		/* 25 Action5 <- <{ p.addValue(buffer[begin:end]); p.addOperator(typeKeyLookup); }> */
		nil,
		/* 26 Action6 <- <{ p.addValue(buffer[begin:end]); p.addOperator(typeKeyReverseLookup); }> */
		nil,
		/* 27 Action7 <- <{ p.addValue(buffer[begin:end]); p.addOperator(typeKeyReverseLookupAttr); }> */
		nil,
		/* 28 Action8 <- <{ p.addOperator(typeKeyReverseLookupHint) }> */
		nil,
		/* 29 Action9 <- <{ p.addValue(buffer[begin:end]) }> */
		nil,
	}
	p.rules = _rules
}
