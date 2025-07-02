package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/your-org/your-project/internal/model"
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
			return nil, fmt.Errorf("user with email %s already exists", req.Email)
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

	return user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id int) (*model.User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user with ID %d not found", id)
	}

	return user, nil
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(id int, req *model.UpdateUserRequest) (*model.User, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user with ID %d not found", id)
	}

	// Check if email already exists (if being updated)
	if req.Email != nil && *req.Email != user.Email {
		for _, existingUser := range s.users {
			if existingUser.ID != id && existingUser.Email == *req.Email {
				return nil, fmt.Errorf("user with email %s already exists", *req.Email)
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

	return user, nil
}

// DeleteUser deletes a user by ID
func (s *UserService) DeleteUser(id int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.users[id]; !exists {
		return fmt.Errorf("user with ID %d not found", id)
	}

	delete(s.users, id)
	return nil
}

// ListUsers returns a paginated list of users
func (s *UserService) ListUsers(page, perPage int) (*model.UserListResponse, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Convert map to slice
	allUsers := make([]*model.User, 0, len(s.users))
	for _, user := range s.users {
		allUsers = append(allUsers, user)
	}

	total := len(allUsers)
	totalPages := (total + perPage - 1) / perPage

	// Calculate pagination
	start := (page - 1) * perPage
	end := start + perPage

	if start >= total {
		return &model.UserListResponse{
			Users: []model.User{},
			Meta: model.MetaData{
				Page:       page,
				PerPage:    perPage,
				Total:      total,
				TotalPages: totalPages,
			},
		}, nil
	}

	if end > total {
		end = total
	}

	// Get paginated users
	paginatedUsers := make([]model.User, 0, end-start)
	for i := start; i < end; i++ {
		paginatedUsers = append(paginatedUsers, *allUsers[i])
	}

	return &model.UserListResponse{
		Users: paginatedUsers,
		Meta: model.MetaData{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}
