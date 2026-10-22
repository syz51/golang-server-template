package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/syz51/golang-server-template/internal/model"
)

func createTestUser(t *testing.T, s *UserService, email string) *model.User {
	t.Helper()
	user, err := s.CreateUser(&model.CreateUserRequest{Email: email, FirstName: "John", LastName: "Doe", Age: 25})
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func TestUserSnapshots(t *testing.T) {
	s := NewUserService()
	created := createTestUser(t, s, "first@example.com")
	created.Email = "mutated@example.com"
	got, err := s.GetUser(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "first@example.com" {
		t.Fatalf("create leaked stored user: %+v", got)
	}
	got.FirstName = "mutated"
	got.LastName = "also mutated"
	stored, err := s.GetUser(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.FirstName != "John" || stored.LastName != "Doe" {
		t.Fatalf("get leaked stored user: %+v", stored)
	}
	firstName := "Updated"
	updated, err := s.UpdateUser(created.ID, &model.UpdateUserRequest{FirstName: &firstName})
	if err != nil {
		t.Fatal(err)
	}
	updated.Email = "mutated@example.com"
	again, err := s.GetUser(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Email != "first@example.com" || again.FirstName != "Updated" {
		t.Fatalf("update leaked stored user: %+v", again)
	}
	if created.FirstName != "John" || got.FirstName != "mutated" {
		t.Fatal("previous snapshots changed on update")
	}
	list, err := s.ListUsers(1, 10)
	if err != nil {
		t.Fatal(err)
	}
	list.Users[0].Email = "list-mutated@example.com"
	again, _ = s.GetUser(created.ID)
	if again.Email != "first@example.com" {
		t.Fatal("list leaked stored user")
	}
}

func TestUserErrors(t *testing.T) {
	s := NewUserService()
	first := createTestUser(t, s, "first@example.com")
	second := createTestUser(t, s, "second@example.com")
	_, err := s.CreateUser(&model.CreateUserRequest{Email: first.Email})
	if !errors.Is(err, ErrEmailExists) {
		t.Fatalf("create duplicate: %v", err)
	}
	_, err = s.UpdateUser(second.ID, &model.UpdateUserRequest{Email: &first.Email})
	if !errors.Is(err, ErrEmailExists) {
		t.Fatalf("update duplicate: %v", err)
	}
	unchanged, _ := s.GetUser(second.ID)
	if unchanged.Email != second.Email {
		t.Fatal("failed update mutated user")
	}
	_, err = s.GetUser(999)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("get missing: %v", err)
	}
	_, err = s.UpdateUser(999, &model.UpdateUserRequest{})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("update missing: %v", err)
	}
	if err = s.DeleteUser(999); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestListUsersPagination(t *testing.T) {
	s := NewUserService()
	for i := 0; i < 12; i++ {
		createTestUser(t, s, fmt.Sprintf("user%d@example.com", i))
	}
	if err := s.DeleteUser(2); err != nil {
		t.Fatal(err)
	}
	for repeat := 0; repeat < 20; repeat++ {
		list, err := s.ListUsers(2, 4)
		if err != nil {
			t.Fatal(err)
		}
		if list.Meta.Total != 11 || list.Meta.TotalPages != 3 || len(list.Users) != 4 {
			t.Fatalf("bad pagination: %+v", list)
		}
		for i, user := range list.Users {
			if user.ID != i+6 {
				t.Fatalf("unstable ID order: %+v", list.Users)
			}
		}
	}
	maxInt := int(^uint(0) >> 1)
	for _, tc := range []struct{ page, perPage int }{{0, 10}, {-1, 10}, {1, 0}, {1, -1}, {1, 101}, {maxInt, 100}, {maxInt, 2}} {
		if _, err := s.ListUsers(tc.page, tc.perPage); !errors.Is(err, ErrInvalidPagination) {
			t.Errorf("%+v: %v", tc, err)
		}
	}
	for _, tc := range []struct{ page, perPage int }{{4, 4}, {maxInt, 1}, {maxInt/100 + 1, 100}} {
		list, err := s.ListUsers(tc.page, tc.perPage)
		if err != nil {
			t.Fatalf("valid out-of-range %+v: %v", tc, err)
		}
		if list.Users == nil || len(list.Users) != 0 {
			t.Fatalf("expected empty array: %+v", list)
		}
	}
	empty, err := NewUserService().ListUsers(1, 100)
	if err != nil || empty.Meta.TotalPages != 0 || empty.Users == nil {
		t.Fatalf("empty collection: %+v, %v", empty, err)
	}
}
