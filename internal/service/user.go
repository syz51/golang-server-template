package service

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/syz51/golang-server-template/internal/model"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrEmailExists       = errors.New("email already exists")
	ErrInvalidPagination = errors.New("invalid pagination")
)

// UserService handles business logic for users
type UserService struct {
	users  map[int]*model.User
	nextID int
	mutex  sync.RWMutex
}

// NewUserService creates a new user service
func NewUserService() *UserService {
	return &UserService{
		users:  make(map[int]*model.User),
		nextID: 1,
	}
}

// CreateUser creates a new user
func (s *UserService) CreateUser(req *model.CreateUserRequest) (*model.User, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if email already exists
	for _, user := range s.users {
		if user.Email == req.Email {
			return nil, fmt.Errorf("%w: %s", ErrEmailExists, req.Email)
		}
	}

	user := &model.User{
		ID:        s.nextID,
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Age:       req.Age,
		Phone:     req.Phone,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.users[s.nextID] = user
	s.nextID++

	snapshot := *user
	return &snapshot, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id int) (*model.User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("%w: ID %d", ErrUserNotFound, id)
	}

	snapshot := *user
	return &snapshot, nil
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(id int, req *model.UpdateUserRequest) (*model.User, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("%w: ID %d", ErrUserNotFound, id)
	}

	// Check if email already exists (if being updated)
	if req.Email != nil && *req.Email != user.Email {
		for _, existingUser := range s.users {
			if existingUser.ID != id && existingUser.Email == *req.Email {
				return nil, fmt.Errorf("%w: %s", ErrEmailExists, *req.Email)
			}
		}
	}

	// Update fields
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		user.LastName = *req.LastName
	}
	if req.Age != nil {
		user.Age = *req.Age
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}
	if req.Status != nil {
		user.Status = *req.Status
	}

	user.UpdatedAt = time.Now()

	snapshot := *user
	return &snapshot, nil
}

// DeleteUser deletes a user by ID
func (s *UserService) DeleteUser(id int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.users[id]; !exists {
		return fmt.Errorf("%w: ID %d", ErrUserNotFound, id)
	}

	delete(s.users, id)
	return nil
}

// ListUsers returns users ordered by ascending ID. Pages beyond the results are empty.
// Invalid limits and offsets that would overflow int are rejected before multiplication.
func (s *UserService) ListUsers(page, perPage int) (*model.UserListResponse, error) {
	if page < 1 || perPage < 1 || perPage > 100 {
		return nil, fmt.Errorf("%w: page must be positive and per_page must be between 1 and 100", ErrInvalidPagination)
	}
	maxInt := int(^uint(0) >> 1)
	if page-1 > maxInt/perPage {
		return nil, fmt.Errorf("%w: page offset is too large", ErrInvalidPagination)
	}
	start := (page - 1) * perPage

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	total := len(s.users)
	totalPages := total / perPage
	if total%perPage != 0 {
		totalPages++
	}
	response := &model.UserListResponse{
		Users: []model.User{},
		Meta:  model.MetaData{Page: page, PerPage: perPage, Total: total, TotalPages: totalPages},
	}
	if start >= total {
		return response, nil
	}

	ids := make([]int, 0, total)
	for id := range s.users {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	// Bound the addition by total to avoid overflowing start + perPage.
	count := min(perPage, total-start)
	response.Users = make([]model.User, 0, count)
	for _, id := range ids[start : start+count] {
		response.Users = append(response.Users, *s.users[id])
	}
	return response, nil
}
