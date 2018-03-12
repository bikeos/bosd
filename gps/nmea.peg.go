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
	rules  [30]func() bool
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
		/* 2 RMC <- <('R' 'M' 'C' Action1 ',' <fix> Action2 ',' <status> Action3 ',' (<lat> Action4)? ',' ns? ',' (<lon> Action5)? ',' we? ',' (<knots> Action6)? ',' (<track> Action7)? ',' <date> Action8 ',' (<magvar> Action9)? ',' nswe? (',' ('A' / 'D' / 'N'))? <chksum> Action10)> */
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
					position11, tokenIndex11 := position, tokenIndex
					{
						position13 := position
						if !_rules[rulelat]() {
							goto l11
						}
						add(rulePegText, position13)
					}
					if !_rules[ruleAction4]() {
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
					if !_rules[rulens]() {
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
					position16, tokenIndex16 := position, tokenIndex
					{
						position18 := position
						if !_rules[rulelon]() {
							goto l16
						}
						add(rulePegText, position18)
					}
					if !_rules[ruleAction5]() {
						goto l16
					}
					goto l17
				l16:
					position, tokenIndex = position16, tokenIndex16
				}
			l17:
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position19, tokenIndex19 := position, tokenIndex
					if !_rules[rulewe]() {
						goto l19
					}
					goto l20
				l19:
					position, tokenIndex = position19, tokenIndex19
				}
			l20:
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position21, tokenIndex21 := position, tokenIndex
					{
						position23 := position
						if !_rules[ruleknots]() {
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
					goto l7
				}
				position++
				{
					position24, tokenIndex24 := position, tokenIndex
					{
						position26 := position
						if !_rules[ruletrack]() {
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
					goto l7
				}
				position++
				{
					position27 := position
					if !_rules[ruledate]() {
						goto l7
					}
					add(rulePegText, position27)
				}
				if !_rules[ruleAction8]() {
					goto l7
				}
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position28, tokenIndex28 := position, tokenIndex
					{
						position30 := position
						if !_rules[rulemagvar]() {
							goto l28
						}
						add(rulePegText, position30)
					}
					if !_rules[ruleAction9]() {
						goto l28
					}
					goto l29
				l28:
					position, tokenIndex = position28, tokenIndex28
				}
			l29:
				if buffer[position] != rune(',') {
					goto l7
				}
				position++
				{
					position31, tokenIndex31 := position, tokenIndex
					if !_rules[rulenswe]() {
						goto l31
					}
					goto l32
				l31:
					position, tokenIndex = position31, tokenIndex31
				}
			l32:
				{
					position33, tokenIndex33 := position, tokenIndex
					if buffer[position] != rune(',') {
						goto l33
					}
					position++
					{
						position35, tokenIndex35 := position, tokenIndex
						if buffer[position] != rune('A') {
							goto l36
						}
						position++
						goto l35
					l36:
						position, tokenIndex = position35, tokenIndex35
						if buffer[position] != rune('D') {
							goto l37
						}
						position++
						goto l35
					l37:
						position, tokenIndex = position35, tokenIndex35
						if buffer[position] != rune('N') {
							goto l33
						}
						position++
					}
				l35:
					goto l34
				l33:
					position, tokenIndex = position33, tokenIndex33
				}
			l34:
				{
					position38 := position
					if !_rules[rulechksum]() {
						goto l7
					}
					add(rulePegText, position38)
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
			position39, tokenIndex39 := position, tokenIndex
			{
				position40 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l39
				}
				position++
			l41:
				{
					position42, tokenIndex42 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l42
					}
					position++
					goto l41
				l42:
					position, tokenIndex = position42, tokenIndex42
				}
				{
					position43, tokenIndex43 := position, tokenIndex
					if buffer[position] != rune('.') {
						goto l43
					}
					position++
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l43
					}
					position++
				l45:
					{
						position46, tokenIndex46 := position, tokenIndex
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l46
						}
						position++
						goto l45
					l46:
						position, tokenIndex = position46, tokenIndex46
					}
					goto l44
				l43:
					position, tokenIndex = position43, tokenIndex43
				}
			l44:
				add(rulefix, position40)
			}
			return true
		l39:
			position, tokenIndex = position39, tokenIndex39
			return false
		},
		/* 4 status <- <('A' / 'V')> */
		func() bool {
			position47, tokenIndex47 := position, tokenIndex
			{
				position48 := position
				{
					position49, tokenIndex49 := position, tokenIndex
					if buffer[position] != rune('A') {
						goto l50
					}
					position++
					goto l49
				l50:
					position, tokenIndex = position49, tokenIndex49
					if buffer[position] != rune('V') {
						goto l47
					}
					position++
				}
			l49:
				add(rulestatus, position48)
			}
			return true
		l47:
			position, tokenIndex = position47, tokenIndex47
			return false
		},
		/* 5 ns <- <('N' / 'S')> */
		func() bool {
			position51, tokenIndex51 := position, tokenIndex
			{
				position52 := position
				{
					position53, tokenIndex53 := position, tokenIndex
					if buffer[position] != rune('N') {
						goto l54
					}
					position++
					goto l53
				l54:
					position, tokenIndex = position53, tokenIndex53
					if buffer[position] != rune('S') {
						goto l51
					}
					position++
				}
			l53:
				add(rulens, position52)
			}
			return true
		l51:
			position, tokenIndex = position51, tokenIndex51
			return false
		},
		/* 6 we <- <('W' / 'E')> */
		func() bool {
			position55, tokenIndex55 := position, tokenIndex
			{
				position56 := position
				{
					position57, tokenIndex57 := position, tokenIndex
					if buffer[position] != rune('W') {
						goto l58
					}
					position++
					goto l57
				l58:
					position, tokenIndex = position57, tokenIndex57
					if buffer[position] != rune('E') {
						goto l55
					}
					position++
				}
			l57:
				add(rulewe, position56)
			}
			return true
		l55:
			position, tokenIndex = position55, tokenIndex55
			return false
		},
		/* 7 nswe <- <(ns / we)> */
		func() bool {
			position59, tokenIndex59 := position, tokenIndex
			{
				position60 := position
				{
					position61, tokenIndex61 := position, tokenIndex
					if !_rules[rulens]() {
						goto l62
					}
					goto l61
				l62:
					position, tokenIndex = position61, tokenIndex61
					if !_rules[rulewe]() {
						goto l59
					}
				}
			l61:
				add(rulenswe, position60)
			}
			return true
		l59:
			position, tokenIndex = position59, tokenIndex59
			return false
		},
		/* 8 lat <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position63, tokenIndex63 := position, tokenIndex
			{
				position64 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l63
				}
				position++
			l65:
				{
					position66, tokenIndex66 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l66
					}
					position++
					goto l65
				l66:
					position, tokenIndex = position66, tokenIndex66
				}
				if buffer[position] != rune('.') {
					goto l63
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l63
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
				add(rulelat, position64)
			}
			return true
		l63:
			position, tokenIndex = position63, tokenIndex63
			return false
		},
		/* 9 lon <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position69, tokenIndex69 := position, tokenIndex
			{
				position70 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l69
				}
				position++
			l71:
				{
					position72, tokenIndex72 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l72
					}
					position++
					goto l71
				l72:
					position, tokenIndex = position72, tokenIndex72
				}
				if buffer[position] != rune('.') {
					goto l69
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l69
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
				add(rulelon, position70)
			}
			return true
		l69:
			position, tokenIndex = position69, tokenIndex69
			return false
		},
		/* 10 knots <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position75, tokenIndex75 := position, tokenIndex
			{
				position76 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l75
				}
				position++
			l77:
				{
					position78, tokenIndex78 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l78
					}
					position++
					goto l77
				l78:
					position, tokenIndex = position78, tokenIndex78
				}
				if buffer[position] != rune('.') {
					goto l75
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l75
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
				add(ruleknots, position76)
			}
			return true
		l75:
			position, tokenIndex = position75, tokenIndex75
			return false
		},
		/* 11 track <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position81, tokenIndex81 := position, tokenIndex
			{
				position82 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l81
				}
				position++
			l83:
				{
					position84, tokenIndex84 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l84
					}
					position++
					goto l83
				l84:
					position, tokenIndex = position84, tokenIndex84
				}
				if buffer[position] != rune('.') {
					goto l81
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l81
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
				add(ruletrack, position82)
			}
			return true
		l81:
			position, tokenIndex = position81, tokenIndex81
			return false
		},
		/* 12 date <- <[0-9]+> */
		func() bool {
			position87, tokenIndex87 := position, tokenIndex
			{
				position88 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l87
				}
				position++
			l89:
				{
					position90, tokenIndex90 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l90
					}
					position++
					goto l89
				l90:
					position, tokenIndex = position90, tokenIndex90
				}
				add(ruledate, position88)
			}
			return true
		l87:
			position, tokenIndex = position87, tokenIndex87
			return false
		},
		/* 13 magvar <- <([0-9]+ '.' [0-9]+)> */
		func() bool {
			position91, tokenIndex91 := position, tokenIndex
			{
				position92 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l91
				}
				position++
			l93:
				{
					position94, tokenIndex94 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l94
					}
					position++
					goto l93
				l94:
					position, tokenIndex = position94, tokenIndex94
				}
				if buffer[position] != rune('.') {
					goto l91
				}
				position++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l91
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
				add(rulemagvar, position92)
			}
			return true
		l91:
			position, tokenIndex = position91, tokenIndex91
			return false
		},
		/* 14 chksum <- <('*' ([0-9] / [A-F]) ([0-9] / [A-F]))> */
		func() bool {
			position97, tokenIndex97 := position, tokenIndex
			{
				position98 := position
				if buffer[position] != rune('*') {
					goto l97
				}
				position++
				{
					position99, tokenIndex99 := position, tokenIndex
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l100
					}
					position++
					goto l99
				l100:
					position, tokenIndex = position99, tokenIndex99
					if c := buffer[position]; c < rune('A') || c > rune('F') {
						goto l97
					}
					position++
				}
			l99:
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
						goto l97
					}
					position++
				}
			l101:
				add(rulechksum, position98)
			}
			return true
		l97:
			position, tokenIndex = position97, tokenIndex97
			return false
		},
		/* 15 unk <- <((!'\n' .)* Action11)> */
		func() bool {
			position103, tokenIndex103 := position, tokenIndex
			{
				position104 := position
			l105:
				{
					position106, tokenIndex106 := position, tokenIndex
					{
						position107, tokenIndex107 := position, tokenIndex
						if buffer[position] != rune('\n') {
							goto l107
						}
						position++
						goto l106
					l107:
						position, tokenIndex = position107, tokenIndex107
					}
					if !matchDot() {
						goto l106
					}
					goto l105
				l106:
					position, tokenIndex = position106, tokenIndex106
				}
				if !_rules[ruleAction11]() {
					goto l103
				}
				add(ruleunk, position104)
			}
			return true
		l103:
			position, tokenIndex = position103, tokenIndex103
			return false
		},
		nil,
		/* 18 Action0 <- <{ p.nmea = &nmeaLine{text, p.nmea} }> */
		func() bool {
			{
				add(ruleAction0, position)
			}
			return true
		},
		/* 19 Action1 <- <{ p.nmea = &p.rmc }> */
		func() bool {
			{
				add(ruleAction1, position)
			}
			return true
		},
		/* 20 Action2 <- <{ p.rmc.fix = text }> */
		func() bool {
			{
				add(ruleAction2, position)
			}
			return true
		},
		/* 21 Action3 <- <{ p.rmc.status = text }> */
		func() bool {
			{
				add(ruleAction3, position)
			}
			return true
		},
		/* 22 Action4 <- <{ p.rmc.lat = text }> */
		func() bool {
			{
				add(ruleAction4, position)
			}
			return true
		},
		/* 23 Action5 <- <{ p.rmc.lon = text }> */
		func() bool {
			{
				add(ruleAction5, position)
			}
			return true
		},
		/* 24 Action6 <- <{ p.rmc.knots.parse(text) }> */
		func() bool {
			{
				add(ruleAction6, position)
			}
			return true
		},
		/* 25 Action7 <- <{ p.rmc.track = text }> */
		func() bool {
			{
				add(ruleAction7, position)
			}
			return true
		},
		/* 26 Action8 <- <{ p.rmc.date = text }> */
		func() bool {
			{
				add(ruleAction8, position)
			}
			return true
		},
		/* 27 Action9 <- <{ p.rmc.magvar = text }> */
		func() bool {
			{
				add(ruleAction9, position)
			}
			return true
		},
		/* 28 Action10 <- <{ p.rmc.chksum = text }> */
		func() bool {
			{
				add(ruleAction10, position)
			}
			return true
		},
		/* 29 Action11 <- <{ p.nmea = &nmeaBase{} }> */
		func() bool {
			{
				add(ruleAction11, position)
			}
			return true
		},
	}
	p.rules = _rules
}
