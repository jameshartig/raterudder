package server

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jameshartig/raterudder/pkg/log"
	"github.com/jameshartig/raterudder/pkg/types"
)

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request body
	var req struct {
		InviteCode string `json:"inviteCode"`
		JoinSiteID string `json:"joinSiteID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.InviteCode == "" || req.JoinSiteID == "" {
		http.Error(w, "inviteCode and joinSiteID are required", http.StatusBadRequest)
		return
	}

	// Get the authenticated user from context (either existing or new-to-register)
	var userID, email string

	if user, ok := ctx.Value(userContextKey).(types.User); ok {
		userID = user.ID
		email = user.Email
	} else if userToRegister, ok := ctx.Value(userToRegisterContextKey).(types.User); ok {
		userID = userToRegister.ID
		email = userToRegister.Email
	}

	if userID == "" {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	// Look up the site
	site, err := s.storage.GetSite(ctx, req.JoinSiteID)
	if err != nil {
		log.Ctx(ctx).WarnContext(ctx, "join: site not found", slog.String("siteID", req.JoinSiteID), slog.Any("error", err))
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	// Validate invite code using constant-time comparison
	if site.InviteCode == "" || subtle.ConstantTimeCompare([]byte(req.InviteCode), []byte(site.InviteCode)) != 1 {
		log.Ctx(ctx).WarnContext(ctx, "join: invalid invite code", slog.String("siteID", req.JoinSiteID), slog.String("userID", userID))
		http.Error(w, "invalid invite code", http.StatusForbidden)
		return
	}

	// Check if user already has permission on this site
	alreadyJoined := false
	for _, p := range site.Permissions {
		if p.UserID == userID {
			alreadyJoined = true
			break
		}
	}

	// 1. Create or Update User
	isNewUser := false
	if _, ok := ctx.Value(userToRegisterContextKey).(types.User); ok {
		isNewUser = true
	}

	if isNewUser {
		// Create the user with this site
		newUser := types.User{
			ID:      userID,
			Email:   email,
			SiteIDs: []string{req.JoinSiteID},
		}
		if err := s.storage.CreateUser(ctx, newUser); err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "join: failed to create user", slog.String("userID", userID), slog.Any("error", err))
			http.Error(w, "failed to join site", http.StatusInternalServerError)
			return
		}
	} else {
		// Existing user — add site to their list if not already there
		existingUser, err := s.storage.GetUser(ctx, userID)
		if err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "join: failed to get user", slog.Any("error", err))
			http.Error(w, "failed to join site", http.StatusInternalServerError)
			return
		}

		hasSite := false
		for _, s := range existingUser.SiteIDs {
			if s == req.JoinSiteID {
				hasSite = true
				break
			}
		}

		if !hasSite {
			existingUser.SiteIDs = append(existingUser.SiteIDs, req.JoinSiteID)
			if err := s.storage.UpdateUser(ctx, existingUser); err != nil {
				log.Ctx(ctx).ErrorContext(ctx, "join: failed to update user", slog.Any("error", err))
				http.Error(w, "failed to join site", http.StatusInternalServerError)
				return
			}
		}
	}

	// 2. Update Site (Add permission)
	if !alreadyJoined {
		site.Permissions = append(site.Permissions, types.SitePermissions{UserID: userID})
		if err := s.storage.UpdateSite(ctx, req.JoinSiteID, site); err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "join: failed to update site", slog.String("siteID", req.JoinSiteID), slog.Any("error", err))
			http.Error(w, "failed to join site", http.StatusInternalServerError)
			return
		}
	}

	log.Ctx(ctx).InfoContext(ctx, "user joined site", slog.String("siteID", req.JoinSiteID))
	w.WriteHeader(http.StatusOK)
}
