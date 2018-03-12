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
)

var rul3s = [...]string{
	"Unknown",
	"NMEA",
	"cmd",
	"RMC",
	"fix",
	"status",
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

	Buffer string
	buffer []rune
	rules  [27]func() bool
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
			p.nmea = &nmeaLine{text, p.nmea}
		case ruleAction1:
			p.nmea = &p.rmc
		case ruleAction2:
			p.rmc.fix = text
		case ruleAction3:
			p.rmc.status = text
		case ruleAction4:
			p.rmc.lat = text
		case ruleAction5:
			p.rmc.lon = text
		case ruleAction6:
			p.rmc.knots.parse(text)
		case ruleAction7:
			p.rmc.track = text
		case ruleAction8:
			p.rmc.date = text
		case ruleAction9:
			p.rmc.magvar = text
		case ruleAction10:
			p.rmc.chksum = text
		case ruleAction11:
			p.nmea = &nmeaBase{}

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
		/* 1 cmd <- <(RMC / unk)> */
		func() bool {
			position3, tokenIndex3 := position, tokenIndex
			{
				position4 := position
				{
					position5, tokenIndex5 := position, tokenIndex
					if !_rules[ruleRMC]() {
						goto l6
					}
					goto l5
				l6:
					position, tokenIndex = position5, tokenIndex5
					if !_rules[ruleunk]() {
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
		/* 2 RMC <- <('R' 'M' 'C' Action1 ',' <fix> Action2 ',' <status> Action3 ',' <lat> Action4 ',' <lon> Action5 ',' <knots> Action6 ',' <track> Action7 ',' <date> Action8 ',' <magvar> Action9 (',' 'D')? <chksum> Action10)> */
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
				if !_rules[ruleAction1]() {
					goto l7
				}
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
				if !_rules[ruleAction2]() {
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
				if !_rules[ruleAction3]() {
					goto l7
				}
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position11 := position
					if !_rules[rulelat]() {
						goto l7
					}
					add(rulePegText, position11)
				}
				if !_rules[ruleAction4]() {
					goto l7
				}
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position12 := position
					if !_rules[rulelon]() {
						goto l7
					}
					add(rulePegText, position12)
				}
				if !_rules[ruleAction5]() {
					goto l7
				}
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position13 := position
					if !_rules[ruleknots]() {
						goto l7
					}
					add(rulePegText, position13)
				}
				if !_rules[ruleAction6]() {
					goto l7
				}
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position14 := position
					if !_rules[ruletrack]() {
						goto l7
					}
					add(rulePegText, position14)
				}
				if !_rules[ruleAction7]() {
					goto l7
				}
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position15 := position
					if !_rules[ruledate]() {
						goto l7
					}
					add(rulePegText, position15)
				}
				if !_rules[ruleAction8]() {
					goto l7
				}
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position16 := position
					if !_rules[rulemagvar]() {
						goto l7
					}
					add(rulePegText, position16)
				}
				if !_rules[ruleAction9]() {
					goto l7
				}
				{
					position17, tokenIndex17 := position, tokenIndex
					if buffer[position] != rune(',') {
						goto l17
					}
					position++
					if buffer[position] != rune('D') {
						goto l17
					}
					position++
					goto l18
				l17:
					position, tokenIndex = position17, tokenIndex17
				}
			l18:
				{
					position19 := position
					if !_rules[rulechksum]() {
						goto l7
					}
					add(rulePegText, position19)
				}
				if !_rules[ruleAction10]() {
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
			position20, tokenIndex20 := position, tokenIndex
			{
				position21 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l20
				}
				position++
			l22:
				{
					position23, tokenIndex23 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l23
					}
					position++
					goto l22
				l23:
					position, tokenIndex = position23, tokenIndex23
				}
				{
					position24, tokenIndex24 := position, tokenIndex
					if buffer[position] != rune('.') {
						goto l24
					}
					position++
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l24
					}
					position++
				l26:
					{
						position27, tokenIndex27 := position, tokenIndex
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l27
						}
						position++
						goto l26
					l27:
						position, tokenIndex = position27, tokenIndex27
					}
					goto l25
				l24:
					position, tokenIndex = position24, tokenIndex24
				}
			l25:
				add(rulefix, position21)
			}
			return true
		l20:
			position, tokenIndex = position20, tokenIndex20
			return false
		},
		/* 4 status <- <('A' / 'V')> */
		func() bool {
			position28, tokenIndex28 := position, tokenIndex
			{
				position29 := position
				{
					position30, tokenIndex30 := position, tokenIndex
					if buffer[position] != rune('A') {
						goto l31
					}
					position++
					goto l30
				l31:
					position, tokenIndex = position30, tokenIndex30
					if buffer[position] != rune('V') {
						goto l28
					}
					position++
				}
			l30:
				add(rulestatus, position29)
			}
			return true
		l28:
			position, tokenIndex = position28, tokenIndex28
			return false
		},
		/* 5 lat <- <([0-9]+ '.' [0-9]+ ',' ('N' / 'S'))> */
		func() bool {
			position32, tokenIndex32 := position, tokenIndex
			{
				position33 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l32
				}
				position++
			l34:
				{
					position35, tokenIndex35 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l35
					}
					position++
					goto l34
				l35:
					position, tokenIndex = position35, tokenIndex35
				}
				if buffer[position] != rune('.') {
					goto l32
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l32
				}
				position++
			l36:
				{
					position37, tokenIndex37 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l37
					}
					position++
					goto l36
				l37:
					position, tokenIndex = position37, tokenIndex37
				}
				if buffer[position] != rune(',') {
					goto l32
				}
				position++
				{
					position38, tokenIndex38 := position, tokenIndex
					if buffer[position] != rune('N') {
						goto l39
					}
					position++
					goto l38
				l39:
					position, tokenIndex = position38, tokenIndex38
					if buffer[position] != rune('S') {
						goto l32
					}
					position++
				}
			l38:
				add(rulelat, position33)
			}
			return true
		l32:
			position, tokenIndex = position32, tokenIndex32
			return false
		},
		/* 6 lon <- <([0-9]+ '.' [0-9]+ ',' ('W' / 'E'))> */
		func() bool {
			position40, tokenIndex40 := position, tokenIndex
			{
				position41 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l40
				}
				position++
			l42:
				{
					position43, tokenIndex43 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l43
					}
					position++
					goto l42
				l43:
					position, tokenIndex = position43, tokenIndex43
				}
				if buffer[position] != rune('.') {
					goto l40
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l40
				}
				position++
			l44:
				{
					position45, tokenIndex45 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l45
					}
					position++
					goto l44
				l45:
					position, tokenIndex = position45, tokenIndex45
				}
				if buffer[position] != rune(',') {
					goto l40
				}
				position++
				{
					position46, tokenIndex46 := position, tokenIndex
					if buffer[position] != rune('W') {
						goto l47
					}
					position++
					goto l46
				l47:
					position, tokenIndex = position46, tokenIndex46
					if buffer[position] != rune('E') {
						goto l40
					}
					position++
				}
			l46:
				add(rulelon, position41)
			}
			return true
		l40:
			position, tokenIndex = position40, tokenIndex40
			return false
		},
		/* 7 knots <- <([0-9]+ '.' [0-9]+)> */
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
				if buffer[position] != rune('.') {
					goto l48
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l48
				}
				position++
			l52:
				{
					position53, tokenIndex53 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l53
					}
					position++
					goto l52
				l53:
					position, tokenIndex = position53, tokenIndex53
				}
				add(ruleknots, position49)
			}
			return true
		l48:
			position, tokenIndex = position48, tokenIndex48
			return false
		},
		/* 8 track <- <([0-9]+ '.' [0-9]+)?> */
		func() bool {
			{
				position55 := position
				{
					position56, tokenIndex56 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l56
					}
					position++
				l58:
					{
						position59, tokenIndex59 := position, tokenIndex
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l59
						}
						position++
						goto l58
					l59:
						position, tokenIndex = position59, tokenIndex59
					}
					if buffer[position] != rune('.') {
						goto l56
					}
					position++
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l56
					}
					position++
				l60:
					{
						position61, tokenIndex61 := position, tokenIndex
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l61
						}
						position++
						goto l60
					l61:
						position, tokenIndex = position61, tokenIndex61
					}
					goto l57
				l56:
					position, tokenIndex = position56, tokenIndex56
				}
			l57:
				add(ruletrack, position55)
			}
			return true
		},
		/* 9 date <- <[0-9]+> */
		func() bool {
			position62, tokenIndex62 := position, tokenIndex
			{
				position63 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l62
				}
				position++
			l64:
				{
					position65, tokenIndex65 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l65
					}
					position++
					goto l64
				l65:
					position, tokenIndex = position65, tokenIndex65
				}
				add(ruledate, position63)
			}
			return true
		l62:
			position, tokenIndex = position62, tokenIndex62
			return false
		},
		/* 10 magvar <- <(([0-9]+ '.' [0-9]+)? ',' ('N' / 'S' / 'W' / 'E')?)> */
		func() bool {
			position66, tokenIndex66 := position, tokenIndex
			{
				position67 := position
				{
					position68, tokenIndex68 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l68
					}
					position++
				l70:
					{
						position71, tokenIndex71 := position, tokenIndex
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l71
						}
						position++
						goto l70
					l71:
						position, tokenIndex = position71, tokenIndex71
					}
					if buffer[position] != rune('.') {
						goto l68
					}
					position++
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l68
					}
					position++
				l72:
					{
						position73, tokenIndex73 := position, tokenIndex
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l73
						}
						position++
						goto l72
					l73:
						position, tokenIndex = position73, tokenIndex73
					}
					goto l69
				l68:
					position, tokenIndex = position68, tokenIndex68
				}
			l69:
				if buffer[position] != rune(',') {
					goto l66
				}
				position++
				{
					position74, tokenIndex74 := position, tokenIndex
					{
						position76, tokenIndex76 := position, tokenIndex
						if buffer[position] != rune('N') {
							goto l77
						}
						position++
						goto l76
					l77:
						position, tokenIndex = position76, tokenIndex76
						if buffer[position] != rune('S') {
							goto l78
						}
						position++
						goto l76
					l78:
						position, tokenIndex = position76, tokenIndex76
						if buffer[position] != rune('W') {
							goto l79
						}
						position++
						goto l76
					l79:
						position, tokenIndex = position76, tokenIndex76
						if buffer[position] != rune('E') {
							goto l74
						}
						position++
					}
				l76:
					goto l75
				l74:
					position, tokenIndex = position74, tokenIndex74
				}
			l75:
				add(rulemagvar, position67)
			}
			return true
		l66:
			position, tokenIndex = position66, tokenIndex66
			return false
		},
		/* 11 chksum <- <('*' ([0-9] / [A-F]) ([0-9] / [A-F]))> */
		func() bool {
			position80, tokenIndex80 := position, tokenIndex
			{
				position81 := position
				if buffer[position] != rune('*') {
					goto l80
				}
				position++
				{
					position82, tokenIndex82 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l83
					}
					position++
					goto l82
				l83:
					position, tokenIndex = position82, tokenIndex82
					if c := buffer[position]; c < rune('A') || c > rune('F') {
						goto l80
					}
					position++
				}
			l82:
				{
					position84, tokenIndex84 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l85
					}
					position++
					goto l84
				l85:
					position, tokenIndex = position84, tokenIndex84
					if c := buffer[position]; c < rune('A') || c > rune('F') {
						goto l80
					}
					position++
				}
			l84:
				add(rulechksum, position81)
			}
			return true
		l80:
			position, tokenIndex = position80, tokenIndex80
			return false
		},
		/* 12 unk <- <((!'\n' .)* Action11)> */
		func() bool {
			position86, tokenIndex86 := position, tokenIndex
			{
				position87 := position
			l88:
				{
					position89, tokenIndex89 := position, tokenIndex
					{
						position90, tokenIndex90 := position, tokenIndex
						if buffer[position] != rune('\n') {
							goto l90
						}
						position++
						goto l89
					l90:
						position, tokenIndex = position90, tokenIndex90
					}
					if !matchDot() {
						goto l89
					}
					goto l88
				l89:
					position, tokenIndex = position89, tokenIndex89
				}
				if !_rules[ruleAction11]() {
					goto l86
				}
				add(ruleunk, position87)
			}
			return true
		l86:
			position, tokenIndex = position86, tokenIndex86
			return false
		},
		nil,
		/* 15 Action0 <- <{ p.nmea = &nmeaLine{text, p.nmea} }> */
		func() bool {
			{
				add(ruleAction0, position)
			}
			return true
		},
		/* 16 Action1 <- <{ p.nmea = &p.rmc }> */
		func() bool {
			{
				add(ruleAction1, position)
			}
			return true
		},
		/* 17 Action2 <- <{ p.rmc.fix = text }> */
		func() bool {
			{
				add(ruleAction2, position)
			}
			return true
		},
		/* 18 Action3 <- <{ p.rmc.status = text }> */
		func() bool {
			{
				add(ruleAction3, position)
			}
			return true
		},
		/* 19 Action4 <- <{ p.rmc.lat = text }> */
		func() bool {
			{
				add(ruleAction4, position)
			}
			return true
		},
		/* 20 Action5 <- <{ p.rmc.lon = text }> */
		func() bool {
			{
				add(ruleAction5, position)
			}
			return true
		},
		/* 21 Action6 <- <{ p.rmc.knots.parse(text) }> */
		func() bool {
			{
				add(ruleAction6, position)
			}
			return true
		},
		/* 22 Action7 <- <{ p.rmc.track = text }> */
		func() bool {
			{
				add(ruleAction7, position)
			}
			return true
		},
		/* 23 Action8 <- <{ p.rmc.date = text }> */
		func() bool {
			{
				add(ruleAction8, position)
			}
			return true
		},
		/* 24 Action9 <- <{ p.rmc.magvar = text }> */
		func() bool {
			{
				add(ruleAction9, position)
			}
			return true
		},
		/* 25 Action10 <- <{ p.rmc.chksum = text }> */
		func() bool {
			{
				add(ruleAction10, position)
			}
			return true
		},
		/* 26 Action11 <- <{ p.nmea = &nmeaBase{} }> */
		func() bool {
			{
				add(ruleAction11, position)
			}
			return true
		},
	}
	p.rules = _rules
}
