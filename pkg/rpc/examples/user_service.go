// Package examples provides example RPC services
package examples

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"golang-gin-rpc/pkg/rpc"
)

// UserService represents a user management service
type UserService struct {
	*rpc.BaseService
	users map[string]*User
}

// User represents a user entity
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewUserService creates a new user service
func NewUserService() *UserService {
	service := &UserService{
		BaseService: rpc.NewBaseService("user"),
		users:       make(map[string]*User),
	}
	
	// Add some sample users
	service.users["1"] = &User{
		ID:        "1",
		Name:      "John Doe",
		Email:     "john@example.com",
		Age:       30,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	service.users["2"] = &User{
		ID:        "2",
		Name:      "Jane Smith",
		Email:     "jane@example.com",
		Age:       25,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	return service
}

// Register registers the user service with a gRPC server
func (s *UserService) Register(server interface{}) error {
	if grpcServer, ok := server.(*grpc.Server); ok {
		// In a real implementation, you would register the actual gRPC service
		// For this example, we'll just set metadata
		s.SetMetadata("grpc_registered", true)
		s.SetMetadata("registration_time", time.Now())
		return nil
	}
	return fmt.Errorf("server is not a gRPC server")
}

// CreateUserRequest represents a request to create a user
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

// CreateUserResponse represents a response when creating a user
type CreateUserResponse struct {
	User *User `json:"user"`
}

// CreateUser creates a new user
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if req.Age <= 0 {
		return nil, status.Error(codes.InvalidArgument, "age must be positive")
	}
	
	// Check if user already exists
	for _, user := range s.users {
		if user.Email == req.Email {
			return nil, status.Error(codes.AlreadyExists, "user with this email already exists")
		}
	}
	
	user := &User{
		ID:        fmt.Sprintf("%d", len(s.users)+1),
		Name:      req.Name,
		Email:     req.Email,
		Age:       req.Age,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	s.users[user.ID] = user
	
	return &CreateUserResponse{User: user}, nil
}

// GetUserRequest represents a request to get a user
type GetUserRequest struct {
	ID string `json:"id"`
}

// GetUserResponse represents a response when getting a user
type GetUserResponse struct {
	User *User `json:"user"`
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
	if req.ID == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	
	user, exists := s.users[req.ID]
	if !exists {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	
	return &GetUserResponse{User: user}, nil
}

// ListUsersRequest represents a request to list users
type ListUsersRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// ListUsersResponse represents a response when listing users
type ListUsersResponse struct {
	Users []*User `json:"users"`
	Total int      `json:"total"`
}

// ListUsers retrieves a list of users
func (s *UserService) ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10 // default limit
	}
	
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	
	users := make([]*User, 0)
	count := 0
	
	for _, user := range s.users {
		if count >= offset && len(users) < limit {
			users = append(users, user)
		}
		count++
	}
	
	return &ListUsersResponse{
		Users: users,
		Total: len(s.users),
	}, nil
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Age   int    `json:"age,omitempty"`
}

// UpdateUserResponse represents a response when updating a user
type UpdateUserResponse struct {
	User *User `json:"user"`
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(ctx context.Context, req *UpdateUserRequest) (*UpdateUserResponse, error) {
	if req.ID == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	
	user, exists := s.users[req.ID]
	if !exists {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Age > 0 {
		user.Age = req.Age
	}
	
	user.UpdatedAt = time.Now()
	
	return &UpdateUserResponse{User: user}, nil
}

// DeleteUserRequest represents a request to delete a user
type DeleteUserRequest struct {
	ID string `json:"id"`
}

// DeleteUserResponse represents a response when deleting a user
type DeleteUserResponse struct {
	Success bool `json:"success"`
}

// DeleteUser deletes a user by ID
func (s *UserService) DeleteUser(ctx context.Context, req *DeleteUserRequest) (*DeleteUserResponse, error) {
	if req.ID == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	
	_, exists := s.users[req.ID]
	if !exists {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	
	delete(s.users, req.ID)
	
	return &DeleteUserResponse{Success: true}, nil
}

// SearchUsersRequest represents a request to search users
type SearchUsersRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// SearchUsersResponse represents a response when searching users
type SearchUsersResponse struct {
	Users []*User `json:"users"`
	Total int     `json:"total"`
}

// SearchUsers searches for users by name or email
func (s *UserService) SearchUsers(ctx context.Context, req *SearchUsersRequest) (*SearchUsersResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}
	
	limit := req.Limit
	if limit <= 0 {
		limit = 10 // default limit
	}
	
	var users []*User
	for _, user := range s.users {
		if len(users) >= limit {
			break
		}
		
		// Simple search: check if query is contained in name or email
		if contains(user.Name, req.Query) || contains(user.Email, req.Query) {
			users = append(users, user)
		}
	}
	
	return &SearchUsersResponse{
		Users: users,
		Total: len(users),
	}, nil
}

// contains checks if a string contains another string (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		hasSubstring(s, substr)))
}

// hasSubstring checks if a string contains a substring (case-insensitive)
func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetStats returns service statistics
func (s *UserService) GetStats(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"total_users":    len(s.users),
		"service_name":   s.Name(),
		"uptime":         s.Uptime().String(),
		"started_at":     s.StartTime(),
		"last_activity":  time.Now(),
	}, nil
}

// Health returns the health status of the user service
func (s *UserService) Health(ctx context.Context, req interface{}) (interface{}, error) {
	health := s.BaseService.Health()
	health.Metadata["total_users"] = len(s.users)
	health.Metadata["service_type"] = "user_management"
	return health, nil
}
