package memos

import (
	"context"
	"fmt"
	"time"

	"github.com/lithammer/shortuuid/v4"
	"golang.org/x/crypto/bcrypt"

	memosstore "classgo/memos/store"
)

// Client wraps the Memos store for in-process memo operations.
type Client struct {
	store     *memosstore.Store
	creatorID int32 // Memos user ID to create memos as
}

// NewClient creates a client that talks directly to the Memos store.
func NewClient(store *memosstore.Store, creatorID int32) *Client {
	return &Client{store: store, creatorID: creatorID}
}

// Memo represents a memo to create.
type Memo struct {
	Content    string
	Visibility string // "PRIVATE", "PROTECTED", "PUBLIC"
	Pinned     bool
}

// MemoResponse is the result of a memo operation.
type MemoResponse struct {
	Name       string
	UID        string
	Content    string
	Visibility string
	Pinned     bool
}

// CreateMemo creates a new memo in the store.
func (c *Client) CreateMemo(memo Memo) (*MemoResponse, error) {
	ctx := context.Background()

	vis := memosstore.Protected
	switch memo.Visibility {
	case "PUBLIC":
		vis = memosstore.Public
	case "PRIVATE":
		vis = memosstore.Private
	}

	now := time.Now().Unix()
	created, err := c.store.CreateMemo(ctx, &memosstore.Memo{
		UID:        shortuuid.New(),
		CreatorID:  c.creatorID,
		Content:    memo.Content,
		Visibility: vis,
		Pinned:     memo.Pinned,
		CreatedTs:  now,
		UpdatedTs:  now,
	})
	if err != nil {
		return nil, fmt.Errorf("create memo: %w", err)
	}

	return &MemoResponse{
		Name:       fmt.Sprintf("memos/%s", created.UID),
		UID:        created.UID,
		Content:    created.Content,
		Visibility: string(created.Visibility),
		Pinned:     created.Pinned,
	}, nil
}

// ListMemos lists memos. Filter is currently unused for direct store access.
func (c *Client) ListMemos(filter string, pageSize int) ([]MemoResponse, error) {
	ctx := context.Background()

	limit := pageSize
	memos, err := c.store.ListMemos(ctx, &memosstore.FindMemo{
		CreatorID: &c.creatorID,
		Limit:     &limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list memos: %w", err)
	}

	var result []MemoResponse
	for _, m := range memos {
		result = append(result, MemoResponse{
			Name:       fmt.Sprintf("memos/%s", m.UID),
			UID:        m.UID,
			Content:    m.Content,
			Visibility: string(m.Visibility),
			Pinned:     m.Pinned,
		})
	}
	return result, nil
}

// NotifyTaskAssigned creates a memo notification when a task is assigned to a student.
func (c *Client) NotifyTaskAssigned(studentID, studentName, taskName, assignedBy string) {
	content := fmt.Sprintf("#task #%s\n\n**New task assigned:** %s\n\nAssigned to: %s\nAssigned by: %s",
		studentID, taskName, studentName, assignedBy)

	_, err := c.CreateMemo(Memo{
		Content:    content,
		Visibility: "PROTECTED",
		Pinned:     false,
	})
	if err != nil {
		// Log but don't fail — notifications are best-effort
		fmt.Printf("notify task assigned error: %v\n", err)
	}
}

// DeleteMemo deletes a memo by ID.
func (c *Client) DeleteMemo(id int32) error {
	ctx := context.Background()
	return c.store.DeleteMemo(ctx, &memosstore.DeleteMemo{ID: id})
}

// EnsureUser creates a Memos user if they don't exist, or returns the existing user's ID.
// Used to create accounts for students/parents/teachers.
func EnsureUser(store *memosstore.Store, username, nickname, email, password string) (int32, error) {
	ctx := context.Background()

	user, err := store.GetUser(ctx, &memosstore.FindUser{Username: &username})
	if err != nil {
		return 0, err
	}
	if user != nil {
		return user.ID, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	newUser, err := store.CreateUser(ctx, &memosstore.User{
		Username:     username,
		Role:         memosstore.RoleUser,
		Email:        email,
		Nickname:     nickname,
		PasswordHash: string(hash),
	})
	if err != nil {
		return 0, fmt.Errorf("create user: %w", err)
	}
	return newUser.ID, nil
}

// ResetPassword changes a Memos user's password.
func ResetPassword(store *memosstore.Store, username, newPassword string) error {
	ctx := context.Background()

	user, err := store.GetUser(ctx, &memosstore.FindUser{Username: &username})
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user %q not found", username)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	hashStr := string(hash)
	now := time.Now().Unix()
	_, err = store.UpdateUser(ctx, &memosstore.UpdateUser{
		ID:           user.ID,
		PasswordHash: &hashStr,
		UpdatedTs:    &now,
	})
	return err
}

// EnsureAdminUser ensures a Memos admin user exists and returns their ID.
func EnsureAdminUser(store *memosstore.Store, username string) (int32, error) {
	ctx := context.Background()

	// Check if user exists
	user, err := store.GetUser(ctx, &memosstore.FindUser{Username: &username})
	if err != nil {
		return 0, err
	}
	if user != nil {
		return user.ID, nil
	}

	// Create the admin user
	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	passwordHash := string(hash)
	if err != nil {
		return 0, err
	}

	newUser, err := store.CreateUser(ctx, &memosstore.User{
		Username:     username,
		Role:         memosstore.RoleAdmin,
		Email:        "",
		Nickname:     "TutorOS Admin",
		PasswordHash: passwordHash,
	})
	if err != nil {
		return 0, fmt.Errorf("create admin user: %w", err)
	}
	return newUser.ID, nil
}
