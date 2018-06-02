package gps

//go:generate peg -strict nmea.peg

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const endSymbol rune = 1114112

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	ruleNMEA
	rulecmd
	ruleRMC
	rulefix
	rulestatus
	rulens
	rulewe
	rulenswe
	rulelat
	rulelon
	ruleknots
	ruletrack
	ruledate
	rulemagvar
	rulechksum
	ruleunk
	rulePegText
	ruleAction0
	ruleAction1
	ruleAction2
	ruleAction3
	ruleAction4
	ruleAction5
	ruleAction6
	ruleAction7
	ruleAction8
	ruleAction9
	ruleAction10
	ruleAction11
	ruleAction12
	ruleAction13
)

var rul3s = [...]string{
	"Unknown",
	"NMEA",
	"cmd",
	"RMC",
	"fix",
	"status",
	"ns",
	"we",
	"nswe",
	"lat",
	"lon",
	"knots",
	"track",
	"date",
	"magvar",
	"chksum",
	"unk",
	"PegText",
	"Action0",
	"Action1",
	"Action2",
	"Action3",
	"Action4",
	"Action5",
	"Action6",
	"Action7",
	"Action8",
	"Action9",
	"Action10",
	"Action11",
	"Action12",
	"Action13",
}

type token32 struct {
	pegRule
	begin, end uint32
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v", rul3s[t.pegRule], t.begin, t.end)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(pretty bool, buffer string) {
	var print func(node *node32, depth int)
	print = func(node *node32, depth int) {
		for node != nil {
			for c := 0; c < depth; c++ {
				fmt.Printf(" ")
			}
			rule := rul3s[node.pegRule]
			quote := strconv.Quote(string(([]rune(buffer)[node.begin:node.end])))
			if !pretty {
				fmt.Printf("%v %v\n", rule, quote)
			} else {
				fmt.Printf("\x1B[34m%v\x1B[m %v\n", rule, quote)
			}
			if node.up != nil {
				print(node.up, depth+1)
			}
			node = node.next
		}
	}
	print(node, 0)
}

func (node *node32) Print(buffer string) {
	node.print(false, buffer)
}

func (node *node32) PrettyPrint(buffer string) {
	node.print(true, buffer)
}

type tokens32 struct {
	tree []token32
}

func (t *tokens32) Trim(length uint32) {
	t.tree = t.tree[:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) AST() *node32 {
	type element struct {
		node *node32
		down *element
	}
	tokens := t.Tokens()
	var stack *element
	for _, token := range tokens {
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
	if stack != nil {
		return stack.node
	}
	return nil
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	t.AST().Print(buffer)
}

func (t *tokens32) PrettyPrintSyntaxTree(buffer string) {
	t.AST().PrettyPrint(buffer)
}

func (t *tokens32) Add(rule pegRule, begin, end, index uint32) {
	if tree := t.tree; int(index) >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	t.tree[index] = token32{
		pegRule: rule,
		begin:   begin,
		end:     end,
	}
}

func (t *tokens32) Tokens() []token32 {
	return t.tree
}

type nmeaGrammar struct {
	nmea NMEA
	rmc  RMC
	msg  NMEAi

	Buffer string
	buffer []rune
	rules  [32]func() bool
	parse  func(rule ...int) error
	reset  func()
	Pretty bool
	tokens32
}

func (p *nmeaGrammar) Parse(rule ...int) error {
	return p.parse(rule...)
}

func (p *nmeaGrammar) Reset() {
	p.reset()
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer []rune, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer {
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
	p   *nmeaGrammar
	max token32
}

func (e *parseError) Error() string {
	tokens, error := []token32{e.max}, "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.buffer, positions)
	format := "parse error near %v (line %v symbol %v - line %v symbol %v):\n%v\n"
	if e.p.Pretty {
		format = "parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n"
	}
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf(format,
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			strconv.Quote(string(e.p.buffer[begin:end])))
	}

	return error
}

func (p *nmeaGrammar) PrintSyntaxTree() {
	if p.Pretty {
		p.tokens32.PrettyPrintSyntaxTree(p.Buffer)
	} else {
		p.tokens32.PrintSyntaxTree(p.Buffer)
	}
}

func (p *nmeaGrammar) Execute() {
	buffer, _buffer, text, begin, end := p.Buffer, p.buffer, "", 0, 0
	for _, token := range p.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)
			text = string(_buffer[begin:end])

		case ruleAction0:
			p.nmea = NMEA{text, p.msg}
		case ruleAction1:
			r := p.rmc
			p.msg = &r
		case ruleAction2:
			p.msg = &nmeaUnk{}
		case ruleAction3:
			p.rmc.fix = text
		case ruleAction4:
			p.rmc.status = text
		case ruleAction5:
			p.rmc.lat = text
		case ruleAction6:
			p.rmc.ns = text
		case ruleAction7:
			p.rmc.lon = text
		case ruleAction8:
			p.rmc.we = text
		case ruleAction9:
			p.rmc.knots.parse(text)
		case ruleAction10:
			p.rmc.track = text
		case ruleAction11:
			p.rmc.date = text
		case ruleAction12:
			p.rmc.magvar = text
		case ruleAction13:
			p.rmc.chksum = text

		}
	}
	_, _, _, _, _ = buffer, _buffer, text, begin, end
}

func (p *nmeaGrammar) Init() {
	var (
		max                  token32
		position, tokenIndex uint32
		buffer               []rune
	)
	p.reset = func() {
		max = token32{}
		position, tokenIndex = 0, 0

		p.buffer = []rune(p.Buffer)
		if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != endSymbol {
			p.buffer = append(p.buffer, endSymbol)
		}
		buffer = p.buffer
	}
	p.reset()

	_rules := p.rules
	tree := tokens32{tree: make([]token32, math.MaxInt16)}
	p.parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokens32 = tree
		if matches {
			p.Trim(tokenIndex)
			return nil
		}
		return &parseError{p, max}
	}

	add := func(rule pegRule, begin uint32) {
		tree.Add(rule, begin, position, tokenIndex)
		tokenIndex++
		if begin != position && position > max.end {
			max = token32{rule, begin, position}
		}
	}

	matchDot := func() bool {
		if buffer[position] != endSymbol {
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
		/* 0 NMEA <- <(<('$' 'G' 'P' cmd '\n')> Action0)> */
		func() bool {
			position0, tokenIndex0 := position, tokenIndex
			{
				position1 := position
				{
					position2 := position
					if buffer[position] != rune('$') {
						goto l0
					}
					position++
					if buffer[position] != rune('G') {
						goto l0
					}
					position++
					if buffer[position] != rune('P') {
						goto l0
					}
					position++
					if !_rules[rulecmd]() {
						goto l0
					}
					if buffer[position] != rune('\n') {
						goto l0
					}
					position++
					add(rulePegText, position2)
				}
				if !_rules[ruleAction0]() {
					goto l0
				}
				add(ruleNMEA, position1)
			}
			return true
		l0:
			position, tokenIndex = position0, tokenIndex0
			return false
		},
		/* 1 cmd <- <((RMC Action1) / (unk Action2))> */
		func() bool {
			position3, tokenIndex3 := position, tokenIndex
			{
				position4 := position
				{
					position5, tokenIndex5 := position, tokenIndex
					if !_rules[ruleRMC]() {
						goto l6
					}
					if !_rules[ruleAction1]() {
						goto l6
					}
					goto l5
				l6:
					position, tokenIndex = position5, tokenIndex5
					if !_rules[ruleunk]() {
						goto l3
					}
					if !_rules[ruleAction2]() {
						goto l3
					}
				}
			l5:
				add(rulecmd, position4)
			}
			return true
		l3:
			position, tokenIndex = position3, tokenIndex3
			return false
		},
		/* 2 RMC <- <('R' 'M' 'C' ',' <fix> Action3 ',' <status> Action4 ',' (<lat> Action5)? ',' (<ns> Action6)? ',' (<lon> Action7)? ',' (<we> Action8)? ',' (<knots> Action9)? ',' (<track> Action10)? ',' <date> Action11 ',' (<magvar> Action12)? ',' nswe? (',' ('A' / 'D' / 'N'))? <chksum> Action13)> */
		func() bool {
			position7, tokenIndex7 := position, tokenIndex
			{
				position8 := position
				if buffer[position] != rune('R') {
					goto l7
				}
				position++
				if buffer[position] != rune('M') {
					goto l7
				}
				position++
				if buffer[position] != rune('C') {
					goto l7
				}
				position++
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position9 := position
					if !_rules[rulefix]() {
						goto l7
					}
					add(rulePegText, position9)
				}
				if !_rules[ruleAction3]() {
					goto l7
				}
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position10 := position
					if !_rules[rulestatus]() {
						goto l7
					}
					add(rulePegText, position10)
				}
				if !_rules[ruleAction4]() {
					goto l7
				}
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position11, tokenIndex11 := position, tokenIndex
					{
						position13 := position
						if !_rules[rulelat]() {
							goto l11
						}
						add(rulePegText, position13)
					}
					if !_rules[ruleAction5]() {
						goto l11
					}
					goto l12
				l11:
					position, tokenIndex = position11, tokenIndex11
				}
			l12:
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position14, tokenIndex14 := position, tokenIndex
					{
						position16 := position
						if !_rules[rulens]() {
							goto l14
						}
						add(rulePegText, position16)
					}
					if !_rules[ruleAction6]() {
						goto l14
					}
					goto l15
				l14:
					position, tokenIndex = position14, tokenIndex14
				}
			l15:
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position17, tokenIndex17 := position, tokenIndex
					{
						position19 := position
						if !_rules[rulelon]() {
							goto l17
						}
						add(rulePegText, position19)
					}
					if !_rules[ruleAction7]() {
						goto l17
					}
					goto l18
				l17:
					position, tokenIndex = position17, tokenIndex17
				}
			l18:
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position20, tokenIndex20 := position, tokenIndex
					{
						position22 := position
						if !_rules[rulewe]() {
							goto l20
						}
						add(rulePegText, position22)
					}
					if !_rules[ruleAction8]() {
						goto l20
					}
					goto l21
				l20:
					position, tokenIndex = position20, tokenIndex20
				}
			l21:
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position23, tokenIndex23 := position, tokenIndex
					{
						position25 := position
						if !_rules[ruleknots]() {
							goto l23
						}
						add(rulePegText, position25)
					}
					if !_rules[ruleAction9]() {
						goto l23
					}
					goto l24
				l23:
					position, tokenIndex = position23, tokenIndex23
				}
			l24:
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position26, tokenIndex26 := position, tokenIndex
					{
						position28 := position
						if !_rules[ruletrack]() {
							goto l26
						}
						add(rulePegText, position28)
					}
					if !_rules[ruleAction10]() {
						goto l26
					}
					goto l27
				l26:
					position, tokenIndex = position26, tokenIndex26
				}
			l27:
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position29 := position
					if !_rules[ruledate]() {
						goto l7
					}
					add(rulePegText, position29)
				}
				if !_rules[ruleAction11]() {
					goto l7
				}
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position30, tokenIndex30 := position, tokenIndex
					{
						position32 := position
						if !_rules[rulemagvar]() {
							goto l30
						}
						add(rulePegText, position32)
					}
					if !_rules[ruleAction12]() {
						goto l30
					}
					goto l31
				l30:
					position, tokenIndex = position30, tokenIndex30
				}
			l31:
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position33, tokenIndex33 := position, tokenIndex
					if !_rules[rulenswe]() {
						goto l33
					}
					goto l34
				l33:
					position, tokenIndex = position33, tokenIndex33
				}
			l34:
				{
					position35, tokenIndex35 := position, tokenIndex
					if buffer[position] != rune(',') {
						goto l35
					}
					position++
					{
						position37, tokenIndex37 := position, tokenIndex
						if buffer[position] != rune('A') {
							goto l38
						}
						position++
						goto l37
					l38:
						position, tokenIndex = position37, tokenIndex37
						if buffer[position] != rune('D') {
							goto l39
						}
						position++
						goto l37
					l39:
						position, tokenIndex = position37, tokenIndex37
						if buffer[position] != rune('N') {
							goto l35
						}
						position++
					}
				l37:
					goto l36
				l35:
					position, tokenIndex = position35, tokenIndex35
				}
			l36:
				{
					position40 := position
					if !_rules[rulechksum]() {
						goto l7
					}
					add(rulePegText, position40)
				}
				if !_rules[ruleAction13]() {
					goto l7
				}
				add(ruleRMC, position8)
			}
			return true
		l7:
			position, tokenIndex = position7, tokenIndex7
			return false
		},
		/* 3 fix <- <([0-9]+ ('.' [0-9]+)?)> */
		func() bool {
			position41, tokenIndex41 := position, tokenIndex
			{
				position42 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l41
				}
				position++
			l43:
				{
					position44, tokenIndex44 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l44
					}
					position++
					goto l43
				l44:
					position, tokenIndex = position44, tokenIndex44
				}
				{
					position45, tokenIndex45 := position, tokenIndex
					if buffer[position] != rune('.') {
						goto l45
					}
					position++
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l45
					}
					position++
				l47:
					{
						position48, tokenIndex48 := position, tokenIndex
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l48
						}
						position++
						goto l47
					l48:
						position, tokenIndex = position48, tokenIndex48
					}
					goto l46
				l45:
					position, tokenIndex = position45, tokenIndex45
				}
			l46:
				add(rulefix, position42)
			}
			return true
		l41:
			position, tokenIndex = position41, tokenIndex41
			return false
		},
		/* 4 status <- <('A' / 'V')> */
		func() bool {
			position49, tokenIndex49 := position, tokenIndex
			{
				position50 := position
				{
					position51, tokenIndex51 := position, tokenIndex
					if buffer[position] != rune('A') {
						goto l52
					}
					position++
					goto l51
				l52:
					position, tokenIndex = position51, tokenIndex51
					if buffer[position] != rune('V') {
						goto l49
					}
					position++
				}
			l51:
				add(rulestatus, position50)
			}
			return true
		l49:
			position, tokenIndex = position49, tokenIndex49
			return false
		},
		/* 5 ns <- <('N' / 'S')> */
		func() bool {
			position53, tokenIndex53 := position, tokenIndex
			{
				position54 := position
				{
					position55, tokenIndex55 := position, tokenIndex
					if buffer[position] != rune('N') {
						goto l56
					}
					position++
					goto l55
				l56:
					position, tokenIndex = position55, tokenIndex55
					if buffer[position] != rune('S') {
						goto l53
					}
					position++
				}
			l55:
				add(rulens, position54)
			}
			return true
		l53:
			position, tokenIndex = position53, tokenIndex53
			return false
		},
		/* 6 we <- <('W' / 'E')> */
		func() bool {
			position57, tokenIndex57 := position, tokenIndex
			{
				position58 := position
				{
					position59, tokenIndex59 := position, tokenIndex
					if buffer[position] != rune('W') {
						goto l60
					}
					position++
					goto l59
				l60:
					position, tokenIndex = position59, tokenIndex59
					if buffer[position] != rune('E') {
						goto l57
					}
					position++
				}
			l59:
				add(rulewe, position58)
			}
			return true
		l57:
			position, tokenIndex = position57, tokenIndex57
			return false
		},
		/* 7 nswe <- <(ns / we)> */
		func() bool {
			position61, tokenIndex61 := position, tokenIndex
			{
				position62 := position
				{
					position63, tokenIndex63 := position, tokenIndex
					if !_rules[rulens]() {
						goto l64
					}
					goto l63
				l64:
					position, tokenIndex = position63, tokenIndex63
					if !_rules[rulewe]() {
						goto l61
					}
				}
			l63:
				add(rulenswe, position62)
			}
			return true
		l61:
			position, tokenIndex = position61, tokenIndex61
			return false
		},
		/* 8 lat <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position65, tokenIndex65 := position, tokenIndex
			{
				position66 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l65
				}
				position++
			l67:
				{
					position68, tokenIndex68 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l68
					}
					position++
					goto l67
				l68:
					position, tokenIndex = position68, tokenIndex68
				}
				if buffer[position] != rune('.') {
					goto l65
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l65
				}
				position++
			l69:
				{
					position70, tokenIndex70 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l70
					}
					position++
					goto l69
				l70:
					position, tokenIndex = position70, tokenIndex70
				}
				add(rulelat, position66)
			}
			return true
		l65:
			position, tokenIndex = position65, tokenIndex65
			return false
		},
		/* 9 lon <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position71, tokenIndex71 := position, tokenIndex
			{
				position72 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l71
				}
				position++
			l73:
				{
					position74, tokenIndex74 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l74
					}
					position++
					goto l73
				l74:
					position, tokenIndex = position74, tokenIndex74
				}
				if buffer[position] != rune('.') {
					goto l71
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l71
				}
				position++
			l75:
				{
					position76, tokenIndex76 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l76
					}
					position++
					goto l75
				l76:
					position, tokenIndex = position76, tokenIndex76
				}
				add(rulelon, position72)
			}
			return true
		l71:
			position, tokenIndex = position71, tokenIndex71
			return false
		},
		/* 10 knots <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position77, tokenIndex77 := position, tokenIndex
			{
				position78 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l77
				}
				position++
			l79:
				{
					position80, tokenIndex80 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l80
					}
					position++
					goto l79
				l80:
					position, tokenIndex = position80, tokenIndex80
				}
				if buffer[position] != rune('.') {
					goto l77
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l77
				}
				position++
			l81:
				{
					position82, tokenIndex82 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l82
					}
					position++
					goto l81
				l82:
					position, tokenIndex = position82, tokenIndex82
				}
				add(ruleknots, position78)
			}
			return true
		l77:
			position, tokenIndex = position77, tokenIndex77
			return false
		},
		/* 11 track <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position83, tokenIndex83 := position, tokenIndex
			{
				position84 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l83
				}
				position++
			l85:
				{
					position86, tokenIndex86 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l86
					}
					position++
					goto l85
				l86:
					position, tokenIndex = position86, tokenIndex86
				}
				if buffer[position] != rune('.') {
					goto l83
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l83
				}
				position++
			l87:
				{
					position88, tokenIndex88 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l88
					}
					position++
					goto l87
				l88:
					position, tokenIndex = position88, tokenIndex88
				}
				add(ruletrack, position84)
			}
			return true
		l83:
			position, tokenIndex = position83, tokenIndex83
			return false
		},
		/* 12 date <- <[0-9]+> */
		func() bool {
			position89, tokenIndex89 := position, tokenIndex
			{
				position90 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l89
				}
				position++
			l91:
				{
					position92, tokenIndex92 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l92
					}
					position++
					goto l91
				l92:
					position, tokenIndex = position92, tokenIndex92
				}
				add(ruledate, position90)
			}
			return true
		l89:
			position, tokenIndex = position89, tokenIndex89
			return false
		},
		/* 13 magvar <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position93, tokenIndex93 := position, tokenIndex
			{
				position94 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l93
				}
				position++
			l95:
				{
					position96, tokenIndex96 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l96
					}
					position++
					goto l95
				l96:
					position, tokenIndex = position96, tokenIndex96
				}
				if buffer[position] != rune('.') {
					goto l93
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l93
				}
				position++
			l97:
				{
					position98, tokenIndex98 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l98
					}
					position++
					goto l97
				l98:
					position, tokenIndex = position98, tokenIndex98
				}
				add(rulemagvar, position94)
			}
			return true
		l93:
			position, tokenIndex = position93, tokenIndex93
			return false
		},
		/* 14 chksum <- <('*' ([0-9] / [A-F]) ([0-9] / [A-F]))> */
		func() bool {
			position99, tokenIndex99 := position, tokenIndex
			{
				position100 := position
				if buffer[position] != rune('*') {
					goto l99
				}
				position++
				{
					position101, tokenIndex101 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l102
					}
					position++
					goto l101
				l102:
					position, tokenIndex = position101, tokenIndex101
					if c := buffer[position]; c < rune('A') || c > rune('F') {
						goto l99
					}
					position++
				}
			l101:
				{
					position103, tokenIndex103 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l104
					}
					position++
					goto l103
				l104:
					position, tokenIndex = position103, tokenIndex103
					if c := buffer[position]; c < rune('A') || c > rune('F') {
						goto l99
					}
					position++
				}
			l103:
				add(rulechksum, position100)
			}
			return true
		l99:
			position, tokenIndex = position99, tokenIndex99
			return false
		},
		/* 15 unk <- <(!'\n' .)*> */
		func() bool {
			{
				position106 := position
			l107:
				{
					position108, tokenIndex108 := position, tokenIndex
					{
						position109, tokenIndex109 := position, tokenIndex
						if buffer[position] != rune('\n') {
							goto l109
						}
						position++
						goto l108
					l109:
						position, tokenIndex = position109, tokenIndex109
					}
					if !matchDot() {
						goto l108
					}
					goto l107
				l108:
					position, tokenIndex = position108, tokenIndex108
				}
				add(ruleunk, position106)
			}
			return true
		},
		nil,
		/* 18 Action0 <- <{ p.nmea = NMEA{text, p.msg} }> */
		func() bool {
			{
				add(ruleAction0, position)
			}
			return true
		},
		/* 19 Action1 <- <{ r := p.rmc; p.msg = &r }> */
		func() bool {
			{
				add(ruleAction1, position)
			}
			return true
		},
		/* 20 Action2 <- <{ p.msg = &nmeaUnk{} }> */
		func() bool {
			{
				add(ruleAction2, position)
			}
			return true
		},
		/* 21 Action3 <- <{ p.rmc.fix = text }> */
		func() bool {
			{
				add(ruleAction3, position)
			}
			return true
		},
		/* 22 Action4 <- <{ p.rmc.status = text }> */
		func() bool {
			{
				add(ruleAction4, position)
			}
			return true
		},
		/* 23 Action5 <- <{ p.rmc.lat = text }> */
		func() bool {
			{
				add(ruleAction5, position)
			}
			return true
		},
		/* 24 Action6 <- <{ p.rmc.ns = text}> */
		func() bool {
			{
				add(ruleAction6, position)
			}
			return true
		},
		/* 25 Action7 <- <{ p.rmc.lon = text }> */
		func() bool {
			{
				add(ruleAction7, position)
			}
			return true
		},
		/* 26 Action8 <- <{ p.rmc.we = text}> */
		func() bool {
			{
				add(ruleAction8, position)
			}
			return true
		},
		/* 27 Action9 <- <{ p.rmc.knots.parse(text) }> */
		func() bool {
			{
				add(ruleAction9, position)
			}
			return true
		},
		/* 28 Action10 <- <{ p.rmc.track = text }> */
		func() bool {
			{
				add(ruleAction10, position)
			}
			return true
		},
		/* 29 Action11 <- <{ p.rmc.date = text }> */
		func() bool {
			{
				add(ruleAction11, position)
			}
			return true
		},
		/* 30 Action12 <- <{ p.rmc.magvar = text }> */
		func() bool {
			{
				add(ruleAction12, position)
			}
			return true
		},
		/* 31 Action13 <- <{ p.rmc.chksum = text }> */
		func() bool {
			{
				add(ruleAction13, position)
			}
			return true
		},
	}
	p.rules = _rules
}
