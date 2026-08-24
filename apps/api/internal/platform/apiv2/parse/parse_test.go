package parse

import (
	"testing"

	"api/internal/platform/apiv2/problem"
)

func TestBool(t *testing.T) {
	if v, err := Bool("true", "nsfw"); err != nil || !v {
		t.Fatalf("true: %v %v", v, err)
	}
	if v, err := Bool("false", "nsfw"); err != nil || v {
		t.Fatalf("false: %v %v", v, err)
	}
	for _, raw := range []string{"1", "0", "yes", "on", "t", "Y", ""} {
		_, err := Bool(raw, "nsfw")
		if err == nil || err.Code != problem.CodeInvalidParameter {
			t.Errorf("%q: want INVALID_PARAMETER, got %+v", raw, err)
		}
	}
}

func TestLimitDoesNotClamp(t *testing.T) {
	n, err := Limit("", 20, 100)
	if err != nil || n != 20 {
		t.Fatalf("empty: %d %v", n, err)
	}
	n, err = Limit("50", 20, 100)
	if err != nil || n != 50 {
		t.Fatalf("50: %d %v", n, err)
	}
	_, err = Limit("101", 20, 100)
	if err == nil || err.Code != problem.CodeLimitTooLarge {
		t.Fatalf("101: %+v", err)
	}
	_, err = Limit("0", 20, 100)
	if err == nil || err.Code != problem.CodeInvalidParameter {
		t.Fatalf("0: %+v", err)
	}
	_, err = Limit("5oo", 20, 100)
	if err == nil || err.Code != problem.CodeInvalidParameter {
		t.Fatalf("5oo: %+v", err)
	}
}

func TestEnumAndDate(t *testing.T) {
	v, err := Enum("live", "state", []string{"live", "draft"})
	if err != nil || v != "live" {
		t.Fatalf("enum: %s %v", v, err)
	}
	_, err = Enum("submited", "state", []string{"live", "draft"})
	if err == nil || err.Code != problem.CodeUnknownEnumValue {
		t.Fatalf("typo: %+v", err)
	}
	if err.Errors[0].Detail == "" || err.Errors[0].Parameter != "state" {
		t.Fatalf("enum error should list the parameter and legal values: %+v", err.Errors)
	}
	_, err = Date("2011-06-24", "released_after")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Date("2011/06/24", "released_after")
	if err == nil || err.Code != problem.CodeInvalidParameter {
		t.Fatalf("slash date: %+v", err)
	}
	_, err = DateTimeUTC("2026-08-19T02:31:06Z", "created_at")
	if err != nil {
		t.Fatal(err)
	}
	_, err = DateTimeUTC("2026-08-19T02:31:06+08:00", "created_at")
	if err == nil {
		t.Fatal("offset timestamps must be rejected")
	}
}
