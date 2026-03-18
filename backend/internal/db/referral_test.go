// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestReferralRepository_CreateReferral tests inserting a referral row.
func TestReferralRepository_CreateReferral(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}

	ctx := context.Background()
	userRepo := NewUserRepository(pool)
	referralRepo := NewReferralRepository(pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create referrer user
	referrer := &models.User{
		Username:    "referrer_" + suffix,
		DisplayName: "Referrer",
		Email:       "referrer_" + suffix + "@example.com",
		Role:        models.UserRoleUser,
	}
	createdReferrer, err := userRepo.Create(ctx, referrer)
	if err != nil {
		t.Fatalf("Create referrer error = %v", err)
	}

	// Create referred user
	referred := &models.User{
		Username:    "referred_" + suffix,
		DisplayName: "Referred",
		Email:       "referred_" + suffix + "@example.com",
		Role:        models.UserRoleUser,
	}
	createdReferred, err := userRepo.Create(ctx, referred)
	if err != nil {
		t.Fatalf("Create referred error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM referrals WHERE referrer_id = $1 OR referred_id = $2", createdReferrer.ID, createdReferred.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1 OR id = $2", createdReferrer.ID, createdReferred.ID)
	}()

	// Create referral
	err = referralRepo.CreateReferral(ctx, createdReferrer.ID, createdReferred.ID)
	if err != nil {
		t.Fatalf("CreateReferral() error = %v", err)
	}

	// Verify via CountByReferrer
	count, err := referralRepo.CountByReferrer(ctx, createdReferrer.ID)
	if err != nil {
		t.Fatalf("CountByReferrer() error = %v", err)
	}
	if count != 1 {
		t.Errorf("CountByReferrer() = %d, want 1", count)
	}
}

// TestReferralRepository_CreateReferral_DuplicateReferredID tests that inserting a duplicate referred_id fails.
func TestReferralRepository_CreateReferral_DuplicateReferredID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}

	ctx := context.Background()
	userRepo := NewUserRepository(pool)
	referralRepo := NewReferralRepository(pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	referrerA := &models.User{
		Username:    "refA_" + suffix,
		DisplayName: "Referrer A",
		Email:       "refA_" + suffix + "@example.com",
		Role:        models.UserRoleUser,
	}
	createdReferrerA, err := userRepo.Create(ctx, referrerA)
	if err != nil {
		t.Fatalf("Create referrerA error = %v", err)
	}

	referrerB := &models.User{
		Username:    "refB_" + suffix,
		DisplayName: "Referrer B",
		Email:       "refB_" + suffix + "@example.com",
		Role:        models.UserRoleUser,
	}
	createdReferrerB, err := userRepo.Create(ctx, referrerB)
	if err != nil {
		t.Fatalf("Create referrerB error = %v", err)
	}

	referred := &models.User{
		Username:    "reffed_dup_" + suffix,
		DisplayName: "Referred Dup",
		Email:       "reffed_dup_" + suffix + "@example.com",
		Role:        models.UserRoleUser,
	}
	createdReferred, err := userRepo.Create(ctx, referred)
	if err != nil {
		t.Fatalf("Create referred error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM referrals WHERE referred_id = $1", createdReferred.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2, $3)", createdReferrerA.ID, createdReferrerB.ID, createdReferred.ID)
	}()

	// First insert should succeed
	err = referralRepo.CreateReferral(ctx, createdReferrerA.ID, createdReferred.ID)
	if err != nil {
		t.Fatalf("First CreateReferral() error = %v", err)
	}

	// Second insert with same referred_id (different referrer) should fail
	err = referralRepo.CreateReferral(ctx, createdReferrerB.ID, createdReferred.ID)
	if err == nil {
		t.Error("Second CreateReferral() with duplicate referred_id expected error, got nil")
	}
}

// TestReferralRepository_FindUserIDByReferralCode tests looking up a user by referral code.
func TestReferralRepository_FindUserIDByReferralCode(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}

	ctx := context.Background()
	userRepo := NewUserRepository(pool)
	referralRepo := NewReferralRepository(pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	user := &models.User{
		Username:    "codeuser_" + suffix,
		DisplayName: "Code User",
		Email:       "codeuser_" + suffix + "@example.com",
		Role:        models.UserRoleUser,
	}
	createdUser, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create user error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", createdUser.ID)
	}()

	// Retrieve the referral code assigned to this user
	code, err := referralRepo.GetReferralCode(ctx, createdUser.ID)
	if err != nil {
		t.Fatalf("GetReferralCode() error = %v", err)
	}
	if code == "" {
		t.Fatal("GetReferralCode() returned empty code")
	}

	// FindUserIDByReferralCode should return the correct user ID
	foundID, err := referralRepo.FindUserIDByReferralCode(ctx, code)
	if err != nil {
		t.Fatalf("FindUserIDByReferralCode() error = %v", err)
	}
	if foundID != createdUser.ID {
		t.Errorf("FindUserIDByReferralCode() = %v, want %v", foundID, createdUser.ID)
	}
}

// TestReferralRepository_FindUserIDByReferralCode_Unknown tests that an unknown code returns ErrNotFound.
func TestReferralRepository_FindUserIDByReferralCode_Unknown(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}

	ctx := context.Background()
	referralRepo := NewReferralRepository(pool)
	defer pool.Close()

	_, err := referralRepo.FindUserIDByReferralCode(ctx, "XXXXXXXX")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("FindUserIDByReferralCode() error = %v, want ErrNotFound", err)
	}
}
