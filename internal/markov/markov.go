package markov

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	MinOrder     = 1
	MaxOrder     = 4
	MaxNameRunes = 64
)

var (
	ErrTooLong      = errors.New("sample exceeds maximum length")
	ErrNoTransition = errors.New("model has no transition for its current context")
)

const (
	startToken int32 = -1
	endToken   int32 = -2
)

type transition struct {
	token  int32
	weight int
}

// Model is an immutable transition table. Its maps and slices are private and
// never changed after Train returns, so independent requests may sample it
// concurrently with their own random generators.
type Model struct {
	order       int
	transitions map[string][]transition
}

// Train builds an order-N rune model. Each distinct NFC/lowercase spelling has
// equal weight, independent of input order. Source spelling and casing are
// preserved in the transition tokens.
func Train(spellings []string, order int) (*Model, error) {
	if order < MinOrder || order > MaxOrder {
		return nil, fmt.Errorf("order must be between %d and %d", MinOrder, MaxOrder)
	}
	normalized, err := normalizeSpellings(spellings)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 {
		return nil, errors.New("cannot train a model without spellings")
	}

	counts := make(map[string]map[int32]int)
	for _, spelling := range normalized {
		history := make([]int32, order)
		for i := range history {
			history[i] = startToken
		}
		for _, r := range spelling {
			if err := addTransition(counts, history, order, int32(r)); err != nil {
				return nil, err
			}
			history = advance(history, int32(r), order)
		}
		if err := addTransition(counts, history, order, endToken); err != nil {
			return nil, err
		}
	}

	transitions := make(map[string][]transition, len(counts))
	for context, successors := range counts {
		ordered := make([]int32, 0, len(successors))
		for token := range successors {
			ordered = append(ordered, token)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
		values := make([]transition, 0, len(ordered))
		for _, token := range ordered {
			values = append(values, transition{token: token, weight: successors[token]})
		}
		transitions[context] = values
	}
	return &Model{order: order, transitions: transitions}, nil
}

// Order reports the order used to train the model.
func (model *Model) Order() int {
	if model == nil {
		return 0
	}
	return model.order
}

// Sample returns one NFC sample using rng. Sampling is bounded to maxRunes;
// when the next token after maxRunes characters is not END, ErrTooLong is
// returned rather than truncating the candidate.
func (model *Model) Sample(rng *rand.Rand, maxRunes int) (string, error) {
	if model == nil || model.order < MinOrder || model.order > MaxOrder {
		return "", errors.New("model is not initialized")
	}
	if rng == nil {
		return "", errors.New("random generator is nil")
	}
	if maxRunes < 1 || maxRunes > MaxNameRunes {
		return "", fmt.Errorf("maximum length must be between 1 and %d runes", MaxNameRunes)
	}

	history := make([]int32, model.order)
	for i := range history {
		history[i] = startToken
	}
	output := make([]rune, 0, maxRunes)
	for {
		token, err := model.nextToken(history, rng)
		if err != nil {
			return string(output), err
		}
		if token == endToken {
			if len(output) == 0 {
				return "", errors.New("model generated an empty spelling")
			}
			return norm.NFC.String(string(output)), nil
		}
		if token < 0 || !utf8.ValidRune(rune(token)) {
			return string(output), fmt.Errorf("model produced invalid rune token %d", token)
		}
		if len(output) == maxRunes {
			return string(output), ErrTooLong
		}
		output = append(output, rune(token))
		history = advance(history, token, model.order)
	}
}

func (model *Model) nextToken(history []int32, rng *rand.Rand) (int32, error) {
	for contextLength := model.order; contextLength >= 0; contextLength-- {
		values, exists := model.transitions[contextKey(history, contextLength)]
		if !exists || len(values) == 0 {
			continue
		}
		total := 0
		for _, value := range values {
			if value.weight <= 0 || value.weight > int(^uint(0)>>1)-total {
				return 0, errors.New("transition weights overflow or are invalid")
			}
			total += value.weight
		}
		draw := rng.IntN(total)
		for _, value := range values {
			if draw < value.weight {
				return value.token, nil
			}
			draw -= value.weight
		}
		return 0, errors.New("transition weights are inconsistent")
	}
	return 0, ErrNoTransition
}

func normalizeSpellings(spellings []string) ([]string, error) {
	values := make([]string, 0, len(spellings))
	for _, spelling := range spellings {
		if !utf8.ValidString(spelling) {
			return nil, errors.New("training spelling is not valid UTF-8")
		}
		spelling = norm.NFC.String(strings.TrimSpace(spelling))
		if spelling == "" {
			return nil, errors.New("training spelling is empty")
		}
		for _, r := range spelling {
			if unicode.IsControl(r) {
				return nil, fmt.Errorf("training spelling %q contains a control character", spelling)
			}
		}
		values = append(values, spelling)
	}
	sort.Strings(values)
	unique := values[:0]
	seen := make(map[string]struct{}, len(values))
	for _, spelling := range values {
		key := strings.ToLower(spelling)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, spelling)
	}
	return unique, nil
}

func addTransition(counts map[string]map[int32]int, history []int32, order int, next int32) error {
	maximum := min(len(history), order)
	for contextLength := 0; contextLength <= maximum; contextLength++ {
		key := contextKey(history, contextLength)
		if counts[key] == nil {
			counts[key] = make(map[int32]int)
		}
		if counts[key][next] == int(^uint(0)>>1) {
			return errors.New("transition count overflow")
		}
		counts[key][next]++
	}
	return nil
}

func contextKey(history []int32, length int) string {
	if length == 0 {
		return ""
	}
	var encoded [MaxOrder * 4]byte
	start := len(history) - length
	for i, token := range history[start:] {
		binary.LittleEndian.PutUint32(encoded[i*4:], uint32(token))
	}
	return string(encoded[:length*4])
}

func advance(history []int32, token int32, order int) []int32 {
	if len(history) == order {
		copy(history, history[1:])
		history[order-1] = token
		return history
	}
	return append(history, token)
}
