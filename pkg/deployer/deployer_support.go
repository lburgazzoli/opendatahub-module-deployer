package deployer

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func resolveRelatedImages(names []string) (map[string]string, error) {
	v := viper.New()
	v.AutomaticEnv()
	images := make(map[string]string, len(names))
	for _, name := range names {
		image := v.GetString(name)
		if strings.TrimSpace(image) == "" {
			return nil, fmt.Errorf("related image environment variable %q is empty or unset", name)
		}
		images[name] = image
	}
	return images, nil
}
