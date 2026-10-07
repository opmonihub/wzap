package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/auth"
	"wzap/internal/config"
	"wzap/internal/model"
	"wzap/internal/storage"
)

// seedAdmin creates the initial admin from WZAP_ADMIN_EMAIL and
// WZAP_ADMIN_PASSWORD when the users table is empty. It is a no-op when users
// already exist (never duplicating the admin) and when both admin variables
// are unset. It touches only users: it never creates, adopts or backfills
// instances — every instance already carries its mandatory owner. A
// half-configured seed warns, creates nothing and lets boot
// proceed.
func seedAdmin(ctx context.Context, cfg config.Config, users storage.UserRepository, log zerolog.Logger) error {
	count, err := users.Count(ctx)
	if err != nil {
		return fmt.Errorf("seed admin: count users: %w", err)
	}
	if count > 0 {
		return nil
	}

	email, password := cfg.AdminEmail, cfg.AdminPassword
	switch {
	case email == "" && password == "":
		return nil
	case email == "" || password == "":
		log.Warn().Msg("admin seed skipped: set both WZAP_ADMIN_EMAIL and WZAP_ADMIN_PASSWORD to create the initial admin")
		return nil
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("seed admin: hash password: %w", err)
	}
	if _, err := users.Create(ctx, model.User{
		ID:            uuid.New(),
		Email:         email,
		PasswordHash:  hash,
		Role:          "admin",
		InstanceQuota: cfg.DefaultUserQuota,
	}); err != nil {
		return fmt.Errorf("seed admin: create admin: %w", err)
	}

	log.Info().Str("email", email).Msg("admin seed created initial admin")
	return nil
}
