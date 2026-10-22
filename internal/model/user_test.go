package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUpdatePhoneValidation(t *testing.T) {
	for _, tc := range []struct {
		body    string
		valid   bool
		present bool
		value   string
	}{
		{`{}`, true, false, ""}, {`{"phone":null}`, true, false, ""}, {`{"phone":""}`, true, true, ""},
		{`{"phone":"+1234567890"}`, true, true, "+1234567890"}, {`{"phone":"invalid"}`, false, true, "invalid"},
	} {
		t.Run(tc.body, func(t *testing.T) {
			var req UpdateUserRequest
			if err := json.Unmarshal([]byte(tc.body), &req); err != nil {
				t.Fatal(err)
			}
			if (req.Phone != nil) != tc.present || (req.Phone != nil && *req.Phone != tc.value) {
				t.Fatalf("unexpected phone: %+v", req.Phone)
			}
			if err := ValidateStruct(&req); (err == nil) != tc.valid {
				t.Fatalf("validation: %v", err)
			}
		})
	}
}

func TestValidationMessages(t *testing.T) {
	age := 151
	first := "X"
	req := UpdateUserRequest{Age: &age, FirstName: &first}
	details := GetValidationErrors(ValidateStruct(&req))
	if details["age"] != "Must be at most 150" || details["first_name"] != "Must be at least 2 characters long" {
		t.Fatalf("details: %+v", details)
	}
	age = -1
	details = GetValidationErrors(ValidateStruct(&req))
	if details["age"] != "Must be at least 1" {
		t.Fatalf("details: %+v", details)
	}
	for _, body := range []string{`{"age":0}`, `{"email":""}`, `{"first_name":""}`, `{"last_name":""}`, `{"status":""}`} {
		var update UpdateUserRequest
		if err := json.Unmarshal([]byte(body), &update); err != nil {
			t.Fatal(err)
		}
		if err := ValidateStruct(&update); err == nil {
			t.Errorf("accepted explicit zero value: %s", body)
		}
	}
	create := CreateUserRequest{Email: "invalid", FirstName: "X", LastName: "Y", Age: 0}
	details = GetValidationErrors(ValidateStruct(&create))
	if details["email"] == "" || details["first_name"] == "" || details["age"] == "" {
		t.Fatalf("missing JSON field names: %+v", details)
	}
	if strings.Contains(details["age"], "characters") {
		t.Fatal("age described as string length")
	}
}
