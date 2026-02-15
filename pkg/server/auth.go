package server

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jameshartig/raterudder/pkg/log"
	"github.com/jameshartig/raterudder/pkg/storage"
	"github.com/jameshartig/raterudder/pkg/types"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = log.With(ctx, log.Ctx(ctx).With(slog.String("reqPath", r.URL.Path)))

		allowNoLogin := r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/status" || r.URL.Path == "/api/join"
		ignoreUserNotFound := r.URL.Path == "/api/join" || r.URL.Path == "/api/auth/status"
		isUpdatePath := r.URL.Path == "/api/update" || r.URL.Path == "/api/updateSites"

		// extract SiteID
		var siteID string
		if r.Method == http.MethodGet {
			siteID = r.URL.Query().Get("siteID")
		} else {
			// read body to find SiteID
			var bodyBytes []byte
			if r.Body != nil {
				// Limit body size to 1MB to prevent DoS
				r.Body = http.MaxBytesReader(w, r.Body, 1048576)
				var err error
				bodyBytes, err = io.ReadAll(r.Body)
				if err != nil {
					log.Ctx(ctx).ErrorContext(ctx, "failed to read request body", slog.Any("error", err))
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				// restore body for next handler
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// try to unmarshal just the SiteID
			if len(bodyBytes) > 0 {
				var justSiteID struct {
					SiteID string `json:"siteID"`
				}
				err := json.Unmarshal(bodyBytes, &justSiteID)
				if err != nil {
					log.Ctx(ctx).ErrorContext(ctx, "failed to unmarshal request body", slog.Any("error", err))
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				siteID = justSiteID.SiteID
			}
		}

		var email string
		var userID string
		var userFound bool
		var authViaUpdateSpecific bool
		// handle authentication
		if s.bypassAuth {
			ctx = context.WithValue(ctx, userContextKey, types.User{
				ID:      "",
				SiteIDs: []string{types.SiteIDNone},
				Admin:   true,
			})
		} else {
			var authSuccess bool

			// Check /api/update specific auth
			if isUpdatePath {
				authHeader := r.Header.Get("Authorization")
				if authHeader != "" {
					if !strings.HasPrefix(authHeader, "Bearer ") {
						log.Ctx(ctx).ErrorContext(ctx, "invalid auth header", slog.String("header", authHeader))
						http.Error(w, "invalid request", http.StatusBadRequest)
						return
					}
					token := strings.TrimPrefix(authHeader, "Bearer ")
					// audience check
					aud := s.updateSpecificAudience
					if aud == "" {
						aud = s.oidcAudience
					}
					payload, err := s.tokenValidator(ctx, token, aud)
					if err != nil {
						log.Ctx(ctx).WarnContext(ctx, "update token validation failed", slog.Any("error", err))
					} else {
						email = payload.Claims["email"].(string)
						userID = payload.Subject
						if s.updateSpecificEmail != "" && subtle.ConstantTimeCompare([]byte(email), []byte(s.updateSpecificEmail)) == 1 {
							authSuccess = true
							authViaUpdateSpecific = true
						} else {
							log.Ctx(ctx).WarnContext(ctx, "update email mismatch", slog.String("got", email), slog.String("want", s.updateSpecificEmail))
							email = "" // invalid
						}
					}
				}
			}

			// normal user auth (cookie)
			if !authSuccess {
				// 1. Authenticate User
				authCookie, err := r.Cookie(authTokenCookie)
				if err != nil && !errors.Is(err, http.ErrNoCookie) {
					log.Ctx(ctx).ErrorContext(ctx, "failed to get auth cookie", slog.Any("error", err))
					http.Error(w, "missing auth cookie", http.StatusBadRequest)
					return
				}
				if authCookie != nil {
					payload, err := s.tokenValidator(ctx, authCookie.Value, s.oidcAudience)
					if err != nil {
						log.Ctx(ctx).ErrorContext(ctx, "auth token validation failed", slog.Any("error", err))
						http.Error(w, "invalid auth token", http.StatusBadRequest)
						return
					} else {
						email = payload.Claims["email"].(string)
						userID = payload.Subject
						authSuccess = true
					}
				} else if !allowNoLogin {
					log.Ctx(ctx).WarnContext(ctx, "no auth cookie found")
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
			}

			if authSuccess {
				// fetch user
				var user types.User
				if s.singleSite {
					user = types.User{
						ID:      userID,
						Email:   email,
						SiteIDs: []string{types.SiteIDNone},
					}
					for _, admin := range s.adminEmails {
						if user.Email == admin {
							user.Admin = true
							break
						}
					}
				} else if authViaUpdateSpecific {
					// We don't need to fetch the user for update-specific auth
					// The handler doesn't use the user object, and we've already validated the token
				} else {
					var err error
					user, err = s.storage.GetUser(ctx, userID)
					if err != nil {
						if ignoreUserNotFound && errors.Is(err, storage.ErrUserNotFound) {
							log.Ctx(ctx).InfoContext(ctx, "user not found, will register on join", slog.String("userID", userID), slog.String("email", email))
							// Put a stub user in context so the join handler can create it
							ctx = context.WithValue(ctx, userToRegisterContextKey, types.User{
								ID:    userID,
								Email: email,
							})
						} else {
							log.Ctx(ctx).WarnContext(ctx, "user lookup failed", slog.String("userID", userID), slog.String("email", email), slog.Any("error", err))
							http.Error(w, "user lookup failed", http.StatusForbidden)
							return
						}
					} else {
						userFound = true
						// User found, proceed with normal logic
						if siteID == "" && !ignoreUserNotFound {
							if len(user.SiteIDs) == 1 {
								siteID = user.SiteIDs[0]
							} else {
								http.Error(w, "siteID required", http.StatusBadRequest)
								return
							}
						}
					}
				}

				if !s.singleSite && siteID != "" && !authViaUpdateSpecific {
					// 3. Check Permissions
					site, err := s.storage.GetSite(ctx, siteID)
					if err != nil {
						log.Ctx(ctx).WarnContext(ctx, "site lookup failed", slog.String("siteID", siteID), slog.Any("error", err))
						http.Error(w, "site access denied", http.StatusForbidden)
						return
					}

					permFound := false
					for _, p := range site.Permissions {
						if p.UserID == user.ID {
							permFound = true
							user.Admin = true
							break
						}
					}
					if !permFound {
						log.Ctx(ctx).WarnContext(ctx, "user does not have permission for site", slog.String("userID", userID), slog.String("email", email), slog.String("site", siteID))
						http.Error(w, "site access denied", http.StatusForbidden)
						return
					}
				}

				if userFound {
					ctx = context.WithValue(ctx, userContextKey, user)
				}
			} else if !allowNoLogin {
				log.Ctx(ctx).WarnContext(ctx, "unauthenticated request")
				s.clearCookie(w)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		if siteID == "" {
			if s.singleSite {
				siteID = types.SiteIDNone
			} else if !allowNoLogin && !isUpdatePath {
				log.Ctx(ctx).WarnContext(ctx, "siteID required", slog.String("userID", userID))
				http.Error(w, "siteID required", http.StatusBadRequest)
				return
			}
		}

		if userID != "" {
			ctx = log.With(ctx, log.Ctx(ctx).With(slog.String("userID", userID)))
		}
		if siteID != "" {
			ctx = log.With(ctx, log.Ctx(ctx).With(slog.String("siteID", siteID)))
		}

		log.Ctx(ctx).DebugContext(
			ctx,
			"authenticated request",
			slog.String("email", email),
			slog.Bool("userFound", userFound),
		)

		ctx = context.WithValue(ctx, siteIDContextKey, siteID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Parse Parse Form to get the token, expecting JSON body
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	payload, err := s.tokenValidator(r.Context(), req.Token, s.oidcAudience)
	if err != nil {
		log.Ctx(r.Context()).WarnContext(r.Context(), "failed to validate id token", slog.Any("error", err))
		http.Error(w, "invalid id token", http.StatusUnauthorized)
		return
	}

	email, ok := payload.Claims["email"].(string)
	if !ok {
		log.Ctx(r.Context()).WarnContext(r.Context(), "invalid email in id token")
		http.Error(w, "invalid oidc claims", http.StatusUnauthorized)
		return
	}

	log.Ctx(r.Context()).InfoContext(r.Context(), "login token validated successfully", slog.String("email", email), slog.String("subject", payload.Subject))

	// Set the cookie
	http.SetCookie(w, &http.Cookie{
		Name:     authTokenCookie,
		Value:    req.Token,
		Expires:  time.Unix(payload.Expires, 0),
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	})

	w.WriteHeader(http.StatusOK)
}

func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     authTokenCookie,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearCookie(w)
	w.WriteHeader(http.StatusOK)
}

type authStatusResponse struct {
	LoggedIn     bool     `json:"loggedIn"`
	IsAdmin      bool     `json:"isAdmin"`
	Email        string   `json:"email"`
	AuthRequired bool     `json:"authRequired"`
	ClientID     string   `json:"clientID"`
	SiteIDs      []string `json:"siteIDs"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	user, loggedIn := r.Context().Value(userContextKey).(types.User)
	if !loggedIn {
		if userToRegister, ok := r.Context().Value(userToRegisterContextKey).(types.User); ok {
			user = userToRegister
			loggedIn = true
		}
	}

	err := json.NewEncoder(w).Encode(authStatusResponse{
		LoggedIn:     loggedIn,
		IsAdmin:      user.Admin,
		Email:        user.Email,
		AuthRequired: s.oidcAudience != "",
		ClientID:     s.oidcAudience,
		SiteIDs:      user.SiteIDs,
	})
	if err != nil {
		panic(http.ErrAbortHandler)
	}
}
