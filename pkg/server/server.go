package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/levenlabs/go-lflag"
	"github.com/raterudder/raterudder/pkg/controller"
	"github.com/raterudder/raterudder/pkg/ess"
	"github.com/raterudder/raterudder/pkg/log"
	"github.com/raterudder/raterudder/pkg/storage"
	"github.com/raterudder/raterudder/pkg/utility"
	"github.com/raterudder/raterudder/web"
	"google.golang.org/api/idtoken"
)

const authTokenCookie = "auth_token"

type contextKey string

const (
	siteIDContextKey         contextKey = "siteID"
	userContextKey           contextKey = "user"
	userToRegisterContextKey contextKey = "userToRegister"
)

// TokenValidator is a function that validates a Google ID Token.
type TokenValidator func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error)

// Server handles the HTTP API and control logic for the Raterudder system.
// It orchestrates interactions between the utility provider, ESS, and storage.
type Server struct {
	utilities  *utility.Map
	ess        *ess.Map
	storage    storage.Database
	controller *controller.Controller

	listenAddr             string
	devProxy               string
	updateSpecificAudience string
	updateSpecificEmail    string
	tokenValidator         TokenValidator
	httpServer             *http.Server

	adminEmails   []string
	oidcAudience  string
	bypassAuth    bool
	singleSite    bool
	encryptionKey string
}

// Configured initializes the Server with dependencies.
// It uses lflag to register command-line flags for configuration.
func Configured(u *utility.Map, e *ess.Map, s storage.Database) *Server {
	srv := &Server{
		utilities:      u,
		ess:            e,
		storage:        s,
		controller:     controller.NewController(),
		tokenValidator: idtoken.Validate,
	}

	// get the port from PORT when running in cloud run
	port := os.Getenv("PORT")
	if port == "" {
		// otherwise default to 8080
		port = "8080"
	}

	listenAddr := lflag.String("http-listen", ":"+port, "HTTP server listen address")
	devProxy := lflag.String("dev-proxy", "", "Address of the dev server (e.g. http://localhost:5173)")
	updateSpecificEmail := lflag.String("update-specific-email", "", "email to validate for /api/update")
	adminEmails := lflag.String("admin-emails", "", "comma-delimited list of email addresses allowed to update settings via IAP")
	oidcAudience := lflag.String("oidc-audience", "", "token to use for id tokens audience to validate")
	updateSpecificAudience := lflag.String("update-specific-audience", "", "audience to validate for /api/update")
	singleSite := lflag.Bool("single-site", false, "Enable single-site mode (disables siteID requirement)")
	encryptionKey := lflag.RequiredString("credentials-encryption-key", "Key for encrypting credentials")

	lflag.Do(func() {
		srv.listenAddr = *listenAddr
		srv.devProxy = *devProxy
		srv.updateSpecificEmail = *updateSpecificEmail
		if *adminEmails != "" {
			srv.adminEmails = strings.Split(*adminEmails, ",")
			for i, email := range srv.adminEmails {
				srv.adminEmails[i] = strings.TrimSpace(email)
			}
		}
		srv.oidcAudience = *oidcAudience
		srv.updateSpecificAudience = *updateSpecificAudience
		srv.singleSite = *singleSite

		if len(*encryptionKey) != 32 {
			log.Ctx(context.Background()).Error("credentials-encryption-key must be 32 characters")
			os.Exit(1)
		}
		srv.encryptionKey = *encryptionKey

		if *devProxy != "" && *oidcAudience == "" && *adminEmails == "" {
			srv.bypassAuth = true
		}
	})

	return srv
}

func (s *Server) setupHandler() http.Handler {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("POST /api/update", s.handleUpdate)
	apiMux.HandleFunc("POST /api/updateSites", s.handleUpdateSites)
	apiMux.HandleFunc("GET /api/history/prices", s.handleHistoryPrices)
	apiMux.HandleFunc("GET /api/history/actions", s.handleHistoryActions)
	apiMux.HandleFunc("GET /api/history/savings", s.handleHistorySavings)
	apiMux.HandleFunc("GET /api/settings", s.handleGetSettings)
	apiMux.HandleFunc("POST /api/settings", s.handleUpdateSettings)
	apiMux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	apiMux.HandleFunc("POST /api/auth/login", s.handleLogin)
	apiMux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	apiMux.HandleFunc("GET /api/forecast", s.handleForecast)
	apiMux.HandleFunc("POST /api/join", s.handleJoin)

	mux := http.NewServeMux()
	mux.Handle("/api/", s.authMiddleware(apiMux))

	// serve the web frontend, either from the embedded filesystem or from the dev server
	if s.devProxy != "" {
		u, err := url.Parse(s.devProxy)
		if err != nil {
			panic(fmt.Errorf("invalid dev-proxy url (%s): %w", s.devProxy, err))
		}
		mux.Handle("/", httputil.NewSingleHostReverseProxy(u))
	} else {
		distFS, err := fs.Sub(web.DistFS, "dist")
		if err != nil {
			panic(fmt.Errorf("failed to get web dist fs: %w", err))
		}
		fileServer := http.FileServer(http.FS(distFS))
		mux.Handle("/", s.spaHandler(distFS, fileServer))
	}
	mux.HandleFunc("/healthz", s.handleHealthz)
	return gziphandler.GzipHandler(s.securityHeadersMiddleware(mux))
}

func (s *Server) getSiteID(r *http.Request) string {
	if siteID, ok := r.Context().Value(siteIDContextKey).(string); ok {
		return siteID
	}
	// we want to have a stack trace when this happens
	panic("no siteID in context")
}

// Run starts the HTTP server and blocks until the context is canceled or an error occurs.
// It also handles graceful shutdown when the context is done.
func (s *Server) Run(ctx context.Context) error {
	s.httpServer = &http.Server{
		Addr:         s.listenAddr,
		Handler:      s.setupHandler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// Use a channel to capturing server errors
	errChan := make(chan error, 1)

	go func() {
		log.Ctx(ctx).InfoContext(ctx, "starting server", slog.String("addr", s.listenAddr))
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
	}()

	select {
	case <-ctx.Done():
		// Context canceled, shut down gracefully
		log.Ctx(ctx).InfoContext(ctx, "shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
		return nil
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: msg}); err != nil {
		slog.Warn("failed to write error response", slog.Any("error", err))
		panic(http.ErrAbortHandler)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		panic(http.ErrAbortHandler)
	}
}

func (s *Server) spaHandler(dir fs.FS, h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Default to serving index.html for unknown paths (SPA)
		if r.URL.Path != "/" {
			// Check if the file exists in the filesystem
			f, err := dir.Open(strings.TrimPrefix(r.URL.Path, "/"))
			if err == nil {
				f.Close()
			} else if errors.Is(err, fs.ErrNotExist) {
				// Don't fallback to index.html for .well-known
				if strings.HasPrefix(r.URL.Path, "/.well-known/") {
					// we don't write JSON here because we don't know what file type is expected
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				// If file doesn't exist, serve index.html
				r.URL.Path = "/"
			} else {
				log.Ctx(r.Context()).ErrorContext(r.Context(), "failed to open file", "error", err)
				// we don't write JSON here because we don't know what file type is expected
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}
		// cache SPA files for an hour
		w.Header().Set("Cache-Control", "public, max-age=3600")

		h.ServeHTTP(w, r)
	}
}
