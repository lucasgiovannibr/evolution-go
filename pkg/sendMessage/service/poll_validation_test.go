package send_service

import (
	"strconv"
	"testing"
)

func TestValidatePoll(t *testing.T) {
	many := make([]string, 13)
	for i := range many {
		many[i] = "opt" + strconv.Itoa(i)
	}

	ok := []*PollStruct{
		{Question: "q", Options: []string{"a", "b"}},
		{Question: "q", Options: []string{"a", "b"}, MaxAnswer: 1},
		{Question: "q", Options: []string{"a", "b", "c"}, MaxAnswer: 3},
		{Question: "q", Options: many[:12]},
	}
	for i, p := range ok {
		if err := p.ValidatePoll(); err != nil {
			t.Errorf("case %d must be valid: %v", i, err)
		}
	}

	bad := map[string]*PollStruct{
		"no question":     {Options: []string{"a", "b"}},
		"blank question":  {Question: "  ", Options: []string{"a", "b"}},
		"one option":      {Question: "q", Options: []string{"a"}},
		"13 options":      {Question: "q", Options: many},
		"empty option":    {Question: "q", Options: []string{"a", " "}},
		"repeated option": {Question: "q", Options: []string{"a", "b", "a"}},
		"maxAnswer high":  {Question: "q", Options: []string{"a", "b"}, MaxAnswer: 3},
		"maxAnswer neg":   {Question: "q", Options: []string{"a", "b"}, MaxAnswer: -1},
	}
	for name, p := range bad {
		if err := p.ValidatePoll(); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
}
