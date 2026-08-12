package export

import (
	"encoding/json"
	"fmt"
	"os"

	"spotify-history/internal/spotify"
)

func JSON(path string, events []spotify.Event) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create JSON: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close JSON: %w", closeErr)
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(events); err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}
