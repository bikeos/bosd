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
	ruletalkerId
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
	"talkerId",
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
	rules  [33]func() bool
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
		/* 0 NMEA <- <(<('$' talkerId cmd '\n')> Action0)> */
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
					if !_rules[ruletalkerId]() {
						goto l0
					}
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
		/* 1 talkerId <- <(('G' 'P') / ('G' 'L') / ('G' 'A') / ('B' 'D') / ('G' 'N'))> */
		func() bool {
			position3, tokenIndex3 := position, tokenIndex
			{
				position4 := position
				{
					position5, tokenIndex5 := position, tokenIndex
					if buffer[position] != rune('G') {
						goto l6
					}
					position++
					if buffer[position] != rune('P') {
						goto l6
					}
					position++
					goto l5
				l6:
					position, tokenIndex = position5, tokenIndex5
					if buffer[position] != rune('G') {
						goto l7
					}
					position++
					if buffer[position] != rune('L') {
						goto l7
					}
					position++
					goto l5
				l7:
					position, tokenIndex = position5, tokenIndex5
					if buffer[position] != rune('G') {
						goto l8
					}
					position++
					if buffer[position] != rune('A') {
						goto l8
					}
					position++
					goto l5
				l8:
					position, tokenIndex = position5, tokenIndex5
					if buffer[position] != rune('B') {
						goto l9
					}
					position++
					if buffer[position] != rune('D') {
						goto l9
					}
					position++
					goto l5
				l9:
					position, tokenIndex = position5, tokenIndex5
					if buffer[position] != rune('G') {
						goto l3
					}
					position++
					if buffer[position] != rune('N') {
						goto l3
					}
					position++
				}
			l5:
				add(ruletalkerId, position4)
			}
			return true
		l3:
			position, tokenIndex = position3, tokenIndex3
			return false
		},
		/* 2 cmd <- <((RMC Action1) / (unk Action2))> */
		func() bool {
			position10, tokenIndex10 := position, tokenIndex
			{
				position11 := position
				{
					position12, tokenIndex12 := position, tokenIndex
					if !_rules[ruleRMC]() {
						goto l13
					}
					if !_rules[ruleAction1]() {
						goto l13
					}
					goto l12
				l13:
					position, tokenIndex = position12, tokenIndex12
					if !_rules[ruleunk]() {
						goto l10
					}
					if !_rules[ruleAction2]() {
						goto l10
					}
				}
			l12:
				add(rulecmd, position11)
			}
			return true
		l10:
			position, tokenIndex = position10, tokenIndex10
			return false
		},
		/* 3 RMC <- <('R' 'M' 'C' ',' <fix> Action3 ',' <status> Action4 ',' (<lat> Action5)? ',' (<ns> Action6)? ',' (<lon> Action7)? ',' (<we> Action8)? ',' (<knots> Action9)? ',' (<track> Action10)? ',' <date> Action11 ',' (<magvar> Action12)? ',' nswe? (',' ('A' / 'D' / 'N'))? <chksum> Action13)> */
		func() bool {
			position14, tokenIndex14 := position, tokenIndex
			{
				position15 := position
				if buffer[position] != rune('R') {
					goto l14
				}
				position++
				if buffer[position] != rune('M') {
					goto l14
				}
				position++
				if buffer[position] != rune('C') {
					goto l14
				}
				position++
				if buffer[position] != rune(',') {
					goto l14
				}
				position++
				{
					position16 := position
					if !_rules[rulefix]() {
						goto l14
					}
					add(rulePegText, position16)
				}
				if !_rules[ruleAction3]() {
					goto l14
				}
				if buffer[position] != rune(',') {
					goto l14
				}
				position++
				{
					position17 := position
					if !_rules[rulestatus]() {
						goto l14
					}
					add(rulePegText, position17)
				}
				if !_rules[ruleAction4]() {
					goto l14
				}
				if buffer[position] != rune(',') {
					goto l14
				}
				position++
				{
					position18, tokenIndex18 := position, tokenIndex
					{
						position20 := position
						if !_rules[rulelat]() {
							goto l18
						}
						add(rulePegText, position20)
					}
					if !_rules[ruleAction5]() {
						goto l18
					}
					goto l19
				l18:
					position, tokenIndex = position18, tokenIndex18
				}
			l19:
				if buffer[position] != rune(',') {
					goto l14
				}
				position++
				{
					position21, tokenIndex21 := position, tokenIndex
					{
						position23 := position
						if !_rules[rulens]() {
							goto l21
						}
						add(rulePegText, position23)
					}
					if !_rules[ruleAction6]() {
						goto l21
					}
					goto l22
				l21:
					position, tokenIndex = position21, tokenIndex21
				}
			l22:
				if buffer[position] != rune(',') {
					goto l14
				}
				position++
				{
					position24, tokenIndex24 := position, tokenIndex
					{
						position26 := position
						if !_rules[rulelon]() {
							goto l24
						}
						add(rulePegText, position26)
					}
					if !_rules[ruleAction7]() {
						goto l24
					}
					goto l25
				l24:
					position, tokenIndex = position24, tokenIndex24
				}
			l25:
				if buffer[position] != rune(',') {
					goto l14
				}
				position++
				{
					position27, tokenIndex27 := position, tokenIndex
					{
						position29 := position
						if !_rules[rulewe]() {
							goto l27
						}
						add(rulePegText, position29)
					}
					if !_rules[ruleAction8]() {
						goto l27
					}
					goto l28
				l27:
					position, tokenIndex = position27, tokenIndex27
				}
			l28:
				if buffer[position] != rune(',') {
					goto l14
				}
				position++
				{
					position30, tokenIndex30 := position, tokenIndex
					{
						position32 := position
						if !_rules[ruleknots]() {
							goto l30
						}
						add(rulePegText, position32)
					}
					if !_rules[ruleAction9]() {
						goto l30
					}
					goto l31
				l30:
					position, tokenIndex = position30, tokenIndex30
				}
			l31:
				if buffer[position] != rune(',') {
					goto l14
				}
				position++
				{
					position33, tokenIndex33 := position, tokenIndex
					{
						position35 := position
						if !_rules[ruletrack]() {
							goto l33
						}
						add(rulePegText, position35)
					}
					if !_rules[ruleAction10]() {
						goto l33
					}
					goto l34
				l33:
					position, tokenIndex = position33, tokenIndex33
				}
			l34:
				if buffer[position] != rune(',') {
					goto l14
				}
				position++
				{
					position36 := position
					if !_rules[ruledate]() {
						goto l14
					}
					add(rulePegText, position36)
				}
				if !_rules[ruleAction11]() {
					goto l14
				}
				if buffer[position] != rune(',') {
					goto l14
				}
				position++
				{
					position37, tokenIndex37 := position, tokenIndex
					{
						position39 := position
						if !_rules[rulemagvar]() {
							goto l37
						}
						add(rulePegText, position39)
					}
					if !_rules[ruleAction12]() {
						goto l37
					}
					goto l38
				l37:
					position, tokenIndex = position37, tokenIndex37
				}
			l38:
				if buffer[position] != rune(',') {
					goto l14
				}
				position++
				{
					position40, tokenIndex40 := position, tokenIndex
					if !_rules[rulenswe]() {
						goto l40
					}
					goto l41
				l40:
					position, tokenIndex = position40, tokenIndex40
				}
			l41:
				{
					position42, tokenIndex42 := position, tokenIndex
					if buffer[position] != rune(',') {
						goto l42
					}
					position++
					{
						position44, tokenIndex44 := position, tokenIndex
						if buffer[position] != rune('A') {
							goto l45
						}
						position++
						goto l44
					l45:
						position, tokenIndex = position44, tokenIndex44
						if buffer[position] != rune('D') {
							goto l46
						}
						position++
						goto l44
					l46:
						position, tokenIndex = position44, tokenIndex44
						if buffer[position] != rune('N') {
							goto l42
						}
						position++
					}
				l44:
					goto l43
				l42:
					position, tokenIndex = position42, tokenIndex42
				}
			l43:
				{
					position47 := position
					if !_rules[rulechksum]() {
						goto l14
					}
					add(rulePegText, position47)
				}
				if !_rules[ruleAction13]() {
					goto l14
				}
				add(ruleRMC, position15)
			}
			return true
		l14:
			position, tokenIndex = position14, tokenIndex14
			return false
		},
		/* 4 fix <- <([0-9]+ ('.' [0-9]+)?)> */
		func() bool {
			position48, tokenIndex48 := position, tokenIndex
			{
				position49 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l48
				}
				position++
			l50:
				{
					position51, tokenIndex51 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l51
					}
					position++
					goto l50
				l51:
					position, tokenIndex = position51, tokenIndex51
				}
				{
					position52, tokenIndex52 := position, tokenIndex
					if buffer[position] != rune('.') {
						goto l52
					}
					position++
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l52
					}
					position++
				l54:
					{
						position55, tokenIndex55 := position, tokenIndex
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l55
						}
						position++
						goto l54
					l55:
						position, tokenIndex = position55, tokenIndex55
					}
					goto l53
				l52:
					position, tokenIndex = position52, tokenIndex52
				}
			l53:
				add(rulefix, position49)
			}
			return true
		l48:
			position, tokenIndex = position48, tokenIndex48
			return false
		},
		/* 5 status <- <('A' / 'V')> */
		func() bool {
			position56, tokenIndex56 := position, tokenIndex
			{
				position57 := position
				{
					position58, tokenIndex58 := position, tokenIndex
					if buffer[position] != rune('A') {
						goto l59
					}
					position++
					goto l58
				l59:
					position, tokenIndex = position58, tokenIndex58
					if buffer[position] != rune('V') {
						goto l56
					}
					position++
				}
			l58:
				add(rulestatus, position57)
			}
			return true
		l56:
			position, tokenIndex = position56, tokenIndex56
			return false
		},
		/* 6 ns <- <('N' / 'S')> */
		func() bool {
			position60, tokenIndex60 := position, tokenIndex
			{
				position61 := position
				{
					position62, tokenIndex62 := position, tokenIndex
					if buffer[position] != rune('N') {
						goto l63
					}
					position++
					goto l62
				l63:
					position, tokenIndex = position62, tokenIndex62
					if buffer[position] != rune('S') {
						goto l60
					}
					position++
				}
			l62:
				add(rulens, position61)
			}
			return true
		l60:
			position, tokenIndex = position60, tokenIndex60
			return false
		},
		/* 7 we <- <('W' / 'E')> */
		func() bool {
			position64, tokenIndex64 := position, tokenIndex
			{
				position65 := position
				{
					position66, tokenIndex66 := position, tokenIndex
					if buffer[position] != rune('W') {
						goto l67
					}
					position++
					goto l66
				l67:
					position, tokenIndex = position66, tokenIndex66
					if buffer[position] != rune('E') {
						goto l64
					}
					position++
				}
			l66:
				add(rulewe, position65)
			}
			return true
		l64:
			position, tokenIndex = position64, tokenIndex64
			return false
		},
		/* 8 nswe <- <(ns / we)> */
		func() bool {
			position68, tokenIndex68 := position, tokenIndex
			{
				position69 := position
				{
					position70, tokenIndex70 := position, tokenIndex
					if !_rules[rulens]() {
						goto l71
					}
					goto l70
				l71:
					position, tokenIndex = position70, tokenIndex70
					if !_rules[rulewe]() {
						goto l68
					}
				}
			l70:
				add(rulenswe, position69)
			}
			return true
		l68:
			position, tokenIndex = position68, tokenIndex68
			return false
		},
		/* 9 lat <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position72, tokenIndex72 := position, tokenIndex
			{
				position73 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l72
				}
				position++
			l74:
				{
					position75, tokenIndex75 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l75
					}
					position++
					goto l74
				l75:
					position, tokenIndex = position75, tokenIndex75
				}
				if buffer[position] != rune('.') {
					goto l72
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l72
				}
				position++
			l76:
				{
					position77, tokenIndex77 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l77
					}
					position++
					goto l76
				l77:
					position, tokenIndex = position77, tokenIndex77
				}
				add(rulelat, position73)
			}
			return true
		l72:
			position, tokenIndex = position72, tokenIndex72
			return false
		},
		/* 10 lon <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position78, tokenIndex78 := position, tokenIndex
			{
				position79 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l78
				}
				position++
			l80:
				{
					position81, tokenIndex81 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l81
					}
					position++
					goto l80
				l81:
					position, tokenIndex = position81, tokenIndex81
				}
				if buffer[position] != rune('.') {
					goto l78
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l78
				}
				position++
			l82:
				{
					position83, tokenIndex83 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l83
					}
					position++
					goto l82
				l83:
					position, tokenIndex = position83, tokenIndex83
				}
				add(rulelon, position79)
			}
			return true
		l78:
			position, tokenIndex = position78, tokenIndex78
			return false
		},
		/* 11 knots <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position84, tokenIndex84 := position, tokenIndex
			{
				position85 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l84
				}
				position++
			l86:
				{
					position87, tokenIndex87 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l87
					}
					position++
					goto l86
				l87:
					position, tokenIndex = position87, tokenIndex87
				}
				if buffer[position] != rune('.') {
					goto l84
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l84
				}
				position++
			l88:
				{
					position89, tokenIndex89 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l89
					}
					position++
					goto l88
				l89:
					position, tokenIndex = position89, tokenIndex89
				}
				add(ruleknots, position85)
			}
			return true
		l84:
			position, tokenIndex = position84, tokenIndex84
			return false
		},
		/* 12 track <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position90, tokenIndex90 := position, tokenIndex
			{
				position91 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l90
				}
				position++
			l92:
				{
					position93, tokenIndex93 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l93
					}
					position++
					goto l92
				l93:
					position, tokenIndex = position93, tokenIndex93
				}
				if buffer[position] != rune('.') {
					goto l90
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l90
				}
				position++
			l94:
				{
					position95, tokenIndex95 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l95
					}
					position++
					goto l94
				l95:
					position, tokenIndex = position95, tokenIndex95
				}
				add(ruletrack, position91)
			}
			return true
		l90:
			position, tokenIndex = position90, tokenIndex90
			return false
		},
		/* 13 date <- <[0-9]+> */
		func() bool {
			position96, tokenIndex96 := position, tokenIndex
			{
				position97 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l96
				}
				position++
			l98:
				{
					position99, tokenIndex99 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l99
					}
					position++
					goto l98
				l99:
					position, tokenIndex = position99, tokenIndex99
				}
				add(ruledate, position97)
			}
			return true
		l96:
			position, tokenIndex = position96, tokenIndex96
			return false
		},
		/* 14 magvar <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position100, tokenIndex100 := position, tokenIndex
			{
				position101 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l100
				}
				position++
			l102:
				{
					position103, tokenIndex103 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l103
					}
					position++
					goto l102
				l103:
					position, tokenIndex = position103, tokenIndex103
				}
				if buffer[position] != rune('.') {
					goto l100
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l100
				}
				position++
			l104:
				{
					position105, tokenIndex105 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l105
					}
					position++
					goto l104
				l105:
					position, tokenIndex = position105, tokenIndex105
				}
				add(rulemagvar, position101)
			}
			return true
		l100:
			position, tokenIndex = position100, tokenIndex100
			return false
		},
		/* 15 chksum <- <('*' ([0-9] / [A-F]) ([0-9] / [A-F]))> */
		func() bool {
			position106, tokenIndex106 := position, tokenIndex
			{
				position107 := position
				if buffer[position] != rune('*') {
					goto l106
				}
				position++
				{
					position108, tokenIndex108 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l109
					}
					position++
					goto l108
				l109:
					position, tokenIndex = position108, tokenIndex108
					if c := buffer[position]; c < rune('A') || c > rune('F') {
						goto l106
					}
					position++
				}
			l108:
				{
					position110, tokenIndex110 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l111
					}
					position++
					goto l110
				l111:
					position, tokenIndex = position110, tokenIndex110
					if c := buffer[position]; c < rune('A') || c > rune('F') {
						goto l106
					}
					position++
				}
			l110:
				add(rulechksum, position107)
			}
			return true
		l106:
			position, tokenIndex = position106, tokenIndex106
			return false
		},
		/* 16 unk <- <(!'\n' .)*> */
		func() bool {
			{
				position113 := position
			l114:
				{
					position115, tokenIndex115 := position, tokenIndex
					{
						position116, tokenIndex116 := position, tokenIndex
						if buffer[position] != rune('\n') {
							goto l116
						}
						position++
						goto l115
					l116:
						position, tokenIndex = position116, tokenIndex116
					}
					if !matchDot() {
						goto l115
					}
					goto l114
				l115:
					position, tokenIndex = position115, tokenIndex115
				}
				add(ruleunk, position113)
			}
			return true
		},
		nil,
		/* 19 Action0 <- <{ p.nmea = NMEA{text, p.msg} }> */
		func() bool {
			{
				add(ruleAction0, position)
			}
			return true
		},
		/* 20 Action1 <- <{ r := p.rmc; p.msg = &r }> */
		func() bool {
			{
				add(ruleAction1, position)
			}
			return true
		},
		/* 21 Action2 <- <{ p.msg = &nmeaUnk{} }> */
		func() bool {
			{
				add(ruleAction2, position)
			}
			return true
		},
		/* 22 Action3 <- <{ p.rmc.fix = text }> */
		func() bool {
			{
				add(ruleAction3, position)
			}
			return true
		},
		/* 23 Action4 <- <{ p.rmc.status = text }> */
		func() bool {
			{
				add(ruleAction4, position)
			}
			return true
		},
		/* 24 Action5 <- <{ p.rmc.lat = text }> */
		func() bool {
			{
				add(ruleAction5, position)
			}
			return true
		},
		/* 25 Action6 <- <{ p.rmc.ns = text}> */
		func() bool {
			{
				add(ruleAction6, position)
			}
			return true
		},
		/* 26 Action7 <- <{ p.rmc.lon = text }> */
		func() bool {
			{
				add(ruleAction7, position)
			}
			return true
		},
		/* 27 Action8 <- <{ p.rmc.we = text}> */
		func() bool {
			{
				add(ruleAction8, position)
			}
			return true
		},
		/* 28 Action9 <- <{ p.rmc.knots.parse(text) }> */
		func() bool {
			{
				add(ruleAction9, position)
			}
			return true
		},
		/* 29 Action10 <- <{ p.rmc.track = text }> */
		func() bool {
			{
				add(ruleAction10, position)
			}
			return true
		},
		/* 30 Action11 <- <{ p.rmc.date = text }> */
		func() bool {
			{
				add(ruleAction11, position)
			}
			return true
		},
		/* 31 Action12 <- <{ p.rmc.magvar = text }> */
		func() bool {
			{
				add(ruleAction12, position)
			}
			return true
		},
		/* 32 Action13 <- <{ p.rmc.chksum = text }> */
		func() bool {
			{
				add(ruleAction13, position)
			}
			return true
		},
	}
	p.rules = _rules
}
