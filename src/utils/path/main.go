package path_utils

import (
	"os/user"
	"strings"
)

func ExpandUser(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		expandedPath := strings.Replace(path, "~", usr.HomeDir, 1)
		return expandedPath, nil
	}
	return path, nil
}
