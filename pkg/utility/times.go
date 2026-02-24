package utility

import (
	"fmt"
	"time"
)

var (
	// PJM uses Eastern Time
	// MISO uses Eastern Time
	etLocation = func() *time.Location {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			panic(fmt.Errorf("failed to load eastern time location: %w", err))
		}
		return loc
	}()

	// ComEd uses Central Time
	ctLocation = func() *time.Location {
		loc, err := time.LoadLocation("America/Chicago")
		if err != nil {
			panic(fmt.Errorf("failed to load central time location: %w", err))
		}
		return loc
	}()
)
