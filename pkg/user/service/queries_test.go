package user_service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"go.mau.fi/whatsmeow"
)

// Validation happens before the client is touched (a service without a client would panic).
func TestQueriesValidateBeforeTouchingTheClient(t *testing.T) {
	u := &userService{}
	ctx := context.Background()

	many := make([]string, maxDeviceQueryNumbers+1)
	for i := range many {
		many[i] = "55319999" + strconv.Itoa(10000+i)
	}

	for name, err := range map[string]error{
		"devices empty":    first(u.GetUserDevices(ctx, &DevicesStruct{}, nil)),
		"devices invalid":  first(u.GetUserDevices(ctx, &DevicesStruct{Number: []string{"@@"}}, nil)),
		"devices too many": first(u.GetUserDevices(ctx, &DevicesStruct{Number: many}, nil)),
		"business empty":   firstProfile(u.GetBusinessProfile(ctx, &BusinessProfileStruct{}, nil)),
		"business invalid": firstProfile(u.GetBusinessProfile(ctx, &BusinessProfileStruct{Number: "@@"}, nil)),
	} {
		var invalid *InvalidNumberError
		if err == nil || !errors.As(err, &invalid) {
			t.Errorf("%s: want an InvalidNumberError, got %v", name, err)
		}
	}
}

func first(_ []UserDevice, err error) error { return err }

func firstProfile(_ interface{}, err error) error { return err }

func TestNotFoundError(t *testing.T) {
	var err error = &NotFoundError{Msg: "no profile"}
	var nf *NotFoundError
	if !errors.As(err, &nf) || err.Error() != "no profile" {
		t.Fatal("NotFoundError must be recognisable and carry its message")
	}
}

func TestIsNoBusinessProfile(t *testing.T) {
	for _, err := range []error{
		errors.New("missing jid in business profile"),
		fmt.Errorf("wrapped: %w", whatsmeow.ErrIQNotFound),
		&whatsmeow.ElementMissingError{Tag: "business_profile", In: "response"},
	} {
		if !isNoBusinessProfile(err) {
			t.Errorf("%v must count as "+"no business profile", err)
		}
	}
	if isNoBusinessProfile(errors.New("timed out")) {
		t.Error("a timeout is not a missing profile")
	}
}
