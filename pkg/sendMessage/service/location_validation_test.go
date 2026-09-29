package send_service

import (
	"math"
	"testing"
)

func TestLocationValidate(t *testing.T) {
	ok := []*LocationStruct{
		{Latitude: -19.9, Longitude: -43.9, Name: "n", Address: "a"},
		{Latitude: 0, Longitude: -78.5, Name: "n", Address: "a"}, // on the equator
		{Latitude: 51.4, Longitude: 0, Name: "n", Address: "a"},  // on the Greenwich meridian
		{Latitude: 90, Longitude: 180, Name: "n", Address: "a"},
	}
	for i, l := range ok {
		if err := l.Validate(); err != nil {
			t.Errorf("case %d must be valid: %v", i, err)
		}
	}

	bad := map[string]*LocationStruct{
		"both zero":  {Name: "n", Address: "a"},
		"lat range":  {Latitude: 91, Longitude: 10, Name: "n", Address: "a"},
		"lon range":  {Latitude: 10, Longitude: -181, Name: "n", Address: "a"},
		"nan":        {Latitude: math.NaN(), Longitude: 10, Name: "n", Address: "a"},
		"no address": {Latitude: 10, Longitude: 10, Name: "n"},
		"no name":    {Latitude: 10, Longitude: 10, Address: "a"},
	}
	for name, l := range bad {
		if err := l.Validate(); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
}
