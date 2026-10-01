package config

import "testing"

func TestCheckGlobalApiKey(t *testing.T) {
	cases := []struct {
		name        string
		key         string
		wantErr     bool
		wantWarning bool
	}{
		{"example key from .env.example", "429683C4C977415CAAFCCE10F7D57E11", true, false},
		{"example key in lower case", "429683c4c977415caafcce10f7d57e11", true, false},
		{"placeholder from the compose examples", "sua-chave-api-segura-aqui", true, false},
		{"short key", "abc123", false, true},
		{"strong key", "9f2c1d7a4b8e3f60a1d5c7e9b2f4a6c8d0e1f3a5b7c9d1e3f5a7b9c1d3e5f7a9", false, false},
	}
	for _, c := range cases {
		warning, err := CheckGlobalApiKey(c.key)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
		}
		if (warning != "") != c.wantWarning {
			t.Errorf("%s: warning = %q, wantWarning %v", c.name, warning, c.wantWarning)
		}
	}
}
