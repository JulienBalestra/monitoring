package env

import (
	"fmt"
	"os"
)

func DefaultFromEnv(key *string, flag, envvar string) error {
	if *key != "" {
		return nil
	}
	*key = os.Getenv(envvar)
	if *key == "" {
		return fmt.Errorf("flag --%s or envvar %s must be set", flag, envvar)
	}
	return nil
}
