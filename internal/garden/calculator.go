package garden

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

type Token struct {
	Symbol, Denom string
	Decimals      int
}

const MaxCalculationBits = 8192

func checkMagnitude(r *big.Rat) error {
	if r.Num().BitLen() > MaxCalculationBits || r.Denom().BitLen() > MaxCalculationBits {
		return fmt.Errorf("계산 결과가 너무 큽니다 (분자·분모 최대 %d비트)", MaxCalculationBits)
	}
	return nil
}

type tokenParser struct {
	ctx  context.Context
	text string
	at   int
}

func Calculate(expression string) (*big.Rat, error) {
	return CalculateContext(context.Background(), expression)
}
func CalculateContext(ctx context.Context, expression string) (*big.Rat, error) {
	if len(expression) > 2048 {
		return nil, fmt.Errorf("수식은 2048자 이하로 입력하세요")
	}
	p := &tokenParser{text: expression, ctx: ctx}
	r, e := p.expr(0)
	p.space()
	if e == nil && p.at != len(p.text) {
		e = fmt.Errorf("위치 %d: 올바르지 않은 입력", p.at+1)
	}
	return r, e
}
func (p *tokenParser) space() {
	for p.at < len(p.text) && unicode.IsSpace(rune(p.text[p.at])) {
		p.at++
	}
}
func (p *tokenParser) expr(depth int) (*big.Rat, error) {
	r, e := p.term(depth)
	if e != nil {
		return nil, e
	}
	for {
		p.space()
		if p.at == len(p.text) || (p.text[p.at] != '+' && p.text[p.at] != '-') {
			return r, nil
		}
		op := p.text[p.at]
		p.at++
		v, e := p.term(depth)
		if e != nil {
			return nil, e
		}
		if op == '+' {
			r.Add(r, v)
		} else {
			r.Sub(r, v)
		}
		if err := checkMagnitude(r); err != nil {
			return nil, err
		}
		if err := p.ctx.Err(); err != nil {
			return nil, err
		}
	}
}
func (p *tokenParser) term(depth int) (*big.Rat, error) {
	r, e := p.atom(depth)
	if e != nil {
		return nil, e
	}
	for {
		p.space()
		if p.at == len(p.text) || (p.text[p.at] != '*' && p.text[p.at] != '/') {
			return r, nil
		}
		op := p.text[p.at]
		p.at++
		v, e := p.atom(depth)
		if e != nil {
			return nil, e
		}
		if op == '*' {
			r.Mul(r, v)
		} else {
			if v.Sign() == 0 {
				return nil, fmt.Errorf("0으로 나눌 수 없습니다")
			}
			r.Quo(r, v)
		}
		if err := checkMagnitude(r); err != nil {
			return nil, err
		}
		if err := p.ctx.Err(); err != nil {
			return nil, err
		}
	}
}
func (p *tokenParser) atom(depth int) (*big.Rat, error) {
	if err := p.ctx.Err(); err != nil {
		return nil, err
	}
	if depth > 64 {
		return nil, fmt.Errorf("괄호 중첩 한도 초과")
	}
	p.space()
	if p.at >= len(p.text) {
		return nil, fmt.Errorf("숫자가 필요합니다")
	}
	ch := p.text[p.at]
	if ch == '+' || ch == '-' {
		p.at++
		v, e := p.atom(depth + 1)
		if e == nil && ch == '-' {
			v.Neg(v)
		}
		return v, e
	}
	if ch == '(' {
		p.at++
		r, e := p.expr(depth + 1)
		if e != nil {
			return nil, e
		}
		p.space()
		if p.at >= len(p.text) || p.text[p.at] != ')' {
			return nil, fmt.Errorf("닫는 괄호가 필요합니다")
		}
		p.at++
		return r, nil
	}
	start := p.at
	dots, digits := 0, 0
	for p.at < len(p.text) {
		c := p.text[p.at]
		if c == '.' {
			dots++
		} else if c >= '0' && c <= '9' {
			digits++
		} else {
			break
		}
		p.at++
	}
	if dots > 1 || digits == 0 || digits > 256 {
		return nil, fmt.Errorf("숫자 형식 오류 (최대 256자리)")
	}
	if p.at < len(p.text) && (p.text[p.at] == 'e' || p.text[p.at] == 'E') {
		p.at++
		expStart := p.at
		if p.at < len(p.text) && (p.text[p.at] == '+' || p.text[p.at] == '-') {
			p.at++
		}
		for p.at < len(p.text) && p.text[p.at] >= '0' && p.text[p.at] <= '9' {
			p.at++
		}
		n, e := strconv.Atoi(p.text[expStart:p.at])
		if e != nil || n > 1000 || n < -1000 {
			return nil, fmt.Errorf("지수 범위는 -1000..1000입니다")
		}
	}
	v, ok := new(big.Rat).SetString(p.text[start:p.at])
	if !ok {
		return nil, fmt.Errorf("숫자 형식 오류")
	}
	if err := checkMagnitude(v); err != nil {
		return nil, err
	}
	return v, nil
}
func Scale(value *big.Rat, decimals int, toBase bool) (*big.Rat, error) {
	if decimals < 0 || decimals > 36 {
		return nil, fmt.Errorf("decimal 범위는 0..36입니다")
	}
	factor := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	result := new(big.Rat).Set(value)
	if toBase {
		result.Mul(result, factor)
	} else {
		result.Quo(result, factor)
	}
	return result, checkMagnitude(result)
}

// FormatRat returns an exact finite decimal, or an explicit fraction for recurring decimals.
func FormatRat(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String()
	}
	denominator := new(big.Int).Set(r.Denom())
	// Powers of two are available directly; reuse division buffers for fives.
	twos := denominator.TrailingZeroBits()
	denominator.Rsh(denominator, twos)
	five, quotient, remainder := big.NewInt(5), new(big.Int), new(big.Int)
	fives := 0
	for {
		quotient.QuoRem(denominator, five, remainder)
		if remainder.Sign() != 0 {
			break
		}
		denominator, quotient = quotient, denominator
		fives++
	}
	digits := max(int(twos), fives)
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return r.RatString() + "  (≈ " + r.FloatString(18) + ")"
	}
	return strings.TrimRight(strings.TrimRight(r.FloatString(digits), "0"), ".")
}
