package config

import (
	"testing"
	"time"
)

func TestDBPoolFromEnv(t *testing.T) {
	p := DBPoolFromEnv()
	if p.MaxOpen != 25 || p.MaxIdle != 10 || p.MaxLifetime != 5*time.Minute || p.MaxIdleTime != time.Minute {
		t.Fatalf("defaults: %+v", p)
	}

	t.Setenv("DB_MAX_OPEN_CONNS", "8")
	t.Setenv("DB_MAX_IDLE_CONNS", "50") // above the open limit
	t.Setenv("DB_CONN_MAX_LIFETIME_MIN", "abc")
	t.Setenv("DB_CONN_MAX_IDLE_MIN", "0") // below the minimum
	p = DBPoolFromEnv()
	if p.MaxOpen != 8 || p.MaxIdle != 8 || p.MaxLifetime != 5*time.Minute || p.MaxIdleTime != time.Minute {
		t.Fatalf("overrides: %+v", p)
	}
}
